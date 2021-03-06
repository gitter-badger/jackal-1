/*
 * Copyright (c) 2018 Miguel Ángel Ortuño.
 * See the LICENSE file for more information.
 */

package server

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ortuman/jackal/config"
	"github.com/ortuman/jackal/log"
	"github.com/ortuman/jackal/module"
	"github.com/ortuman/jackal/server/transport"
	"github.com/ortuman/jackal/stream"
	"github.com/ortuman/jackal/stream/errors"
	"github.com/ortuman/jackal/xml"
	"github.com/pborman/uuid"
)

type streamState uint8

const (
	connecting streamState = iota
	connected
	authenticating
	authenticated
	sessionStarted
	disconnected
)

const (
	jabberClientNamespace = "jabber:client"
	jabberServerNamespace = "jabber:server"
)

const (
	streamNamespace           = "http://etherx.jabber.org/streams"
	tlsNamespace              = "urn:ietf:params:xml:ns:xmpp-tls"
	compressProtocolNamespace = "http://jabber.org/protocol/compress"
	bindNamespace             = "urn:ietf:params:xml:ns:xmpp-bind"
	sessionNamespace          = "urn:ietf:params:xml:ns:xmpp-session"
)

var (
	errResourceNotFound = errors.New("resource not found")
	errNotAuthenticated = errors.New("user not authenticated")
)

type serverStream struct {
	lock          sync.RWMutex
	cfg           *config.Server
	connected     uint32
	tr            transport.Transport
	parser        *xml.Parser
	state         streamState
	id            string
	username      string
	domain        string
	resource      string
	jid           *xml.JID
	secured       bool
	authenticated bool
	compressed    bool
	priority      int8

	authrs      []authenticator
	activeAuthr authenticator

	iqHandlers []module.IQHandler

	roster           *module.ModRoster
	rosterOnce       sync.Once
	available        bool
	presenceElements []xml.Element

	register *module.XEPRegister
	ping     *module.XEPPing

	offline     *module.ModOffline
	offlineOnce sync.Once

	writeCh chan xml.Element
	readCh  chan xml.Element
	discCh  chan error
}

func newSocketStream(id string, conn net.Conn, config *config.Server) *serverStream {
	s := &serverStream{
		cfg:     config,
		id:      id,
		state:   connecting,
		writeCh: make(chan xml.Element, 256),
		readCh:  make(chan xml.Element),
		discCh:  make(chan error),
	}
	// assign default domain
	s.domain = stream.C2S().DefaultDomain()
	s.jid, _ = xml.NewJID("", s.domain, "", true)

	bufferSize := config.Transport.BufferSize
	keepAlive := config.Transport.KeepAlive
	s.tr = transport.NewSocketTransport(conn, bufferSize, keepAlive)
	s.parser = xml.NewParser(s.tr)

	// initialize authenticators
	s.initializeAuthenticators()

	// initialize XEPs
	s.initializeXEPs()

	if config.Transport.ConnectTimeout > 0 {
		s.startConnectTimeoutTimer(config.Transport.ConnectTimeout)
	}
	go s.loop()

	return s
}

func (s *serverStream) ID() string {
	return s.id
}

func (s *serverStream) Username() string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.username
}

func (s *serverStream) Domain() string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.domain
}

func (s *serverStream) Resource() string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.resource
}

func (s *serverStream) JID() *xml.JID {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.jid
}

func (s *serverStream) Priority() int8 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.priority
}

func (s *serverStream) SendElement(element xml.Element) {
	s.writeCh <- element
}

func (s *serverStream) Disconnect(err error) {
	s.discCh <- err
}

func (s *serverStream) IsAuthenticated() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.authenticated
}

func (s *serverStream) IsSecured() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.secured
}

func (s *serverStream) IsCompressed() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.compressed
}

func (s *serverStream) IsRosterRequested() bool {
	if s.roster != nil {
		return s.roster.IsRequested()
	}
	return false
}

func (s *serverStream) PresenceElements() []xml.Element {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.presenceElements
}

func (s *serverStream) initializeAuthenticators() {
	for _, a := range s.cfg.SASL {
		switch a {
		case "plain":
			s.authrs = append(s.authrs, newPlainAuthenticator(s))
		case "digest_md5":
			s.authrs = append(s.authrs, newDigestMD5(s))
		case "scram_sha_1":
			s.authrs = append(s.authrs, newScram(s, s.tr, sha1ScramType, false))
			s.authrs = append(s.authrs, newScram(s, s.tr, sha1ScramType, true))

		case "scram_sha_256":
			s.authrs = append(s.authrs, newScram(s, s.tr, sha256ScramType, false))
			s.authrs = append(s.authrs, newScram(s, s.tr, sha256ScramType, true))
		}
	}
}

func (s *serverStream) initializeXEPs() {
	// Roster (https://xmpp.org/rfcs/rfc3921.html#roster)
	s.roster = module.NewRoster(s)
	s.iqHandlers = append(s.iqHandlers, s.roster)

	// XEP-0030: Service Discovery (https://xmpp.org/extensions/xep-0030.html)
	discoInfo := module.NewXEPDiscoInfo(s)
	s.iqHandlers = append(s.iqHandlers, discoInfo)

	// XEP-0049: Private XML Storage (https://xmpp.org/extensions/xep-0049.html)
	if _, ok := s.cfg.Modules["private"]; ok {
		s.iqHandlers = append(s.iqHandlers, module.NewXEPPrivateStorage(s))
	}

	// XEP-0054: vcard-temp (https://xmpp.org/extensions/xep-0054.html)
	if _, ok := s.cfg.Modules["vcard"]; ok {
		s.iqHandlers = append(s.iqHandlers, module.NewXEPVCard(s))
	}

	// XEP-0077: In-band registration (https://xmpp.org/extensions/xep-0077.html)
	if _, ok := s.cfg.Modules["registration"]; ok {
		s.register = module.NewXEPRegister(&s.cfg.ModRegistration, s)
		s.iqHandlers = append(s.iqHandlers, s.register)
	}

	// XEP-0092: Software Version (https://xmpp.org/extensions/xep-0092.html)
	if _, ok := s.cfg.Modules["version"]; ok {
		s.iqHandlers = append(s.iqHandlers, module.NewXEPVersion(&s.cfg.ModVersion, s))
	}

	// XEP-0199: XMPP Ping (https://xmpp.org/extensions/xep-0199.html)
	if _, ok := s.cfg.Modules["ping"]; ok {
		s.ping = module.NewXEPPing(&s.cfg.ModPing, s)
		s.iqHandlers = append(s.iqHandlers, s.ping)
	}

	// register server disco info identities
	identities := []module.DiscoIdentity{{
		Category: "server",
		Type:     "im",
		Name:     s.cfg.ID,
	}}
	discoInfo.SetIdentities(identities)

	// register disco info features
	var features []string
	for _, iqHandler := range s.iqHandlers {
		features = append(features, iqHandler.AssociatedNamespaces()...)
	}

	// XEP-0160: Offline message storage (https://xmpp.org/extensions/xep-0160.html)
	if _, ok := s.cfg.Modules["offline"]; ok {
		s.offline = module.NewOffline(&s.cfg.ModOffline, s)
		features = append(features, s.offline.AssociatedNamespaces()...)
	}
	discoInfo.SetFeatures(features)
}

func (s *serverStream) startConnectTimeoutTimer(timeoutInSeconds int) {
	go func() {
		tr := time.NewTimer(time.Second * time.Duration(timeoutInSeconds))
		<-tr.C
		if atomic.LoadUint32(&s.connected) == 0 {
			// connection timeout...
			s.discCh <- streamerror.ErrConnectionTimeout
		}
	}()
}

func (s *serverStream) handleElement(elem xml.Element) {
	switch s.state {
	case connecting:
		s.handleConnecting(elem)
	case connected:
		s.handleConnected(elem)
	case authenticated:
		s.handleAuthenticated(elem)
	case authenticating:
		s.handleAuthenticating(elem)
	case sessionStarted:
		s.handleSessionStarted(elem)
	default:
		break
	}
}

func (s *serverStream) handleConnecting(elem xml.Element) {
	// activate 'connected' flag
	atomic.StoreUint32(&s.connected, 1)

	// validate stream element
	if err := s.validateStreamElement(elem); err != nil {
		s.disconnectWithStreamError(err)
		return
	}
	// assign stream domain
	s.lock.Lock()
	s.domain = elem.To()
	s.lock.Unlock()

	// open stream
	s.openStreamElement()

	features := xml.NewElementName("stream:features")
	features.SetAttribute("xmlns:stream", streamNamespace)
	features.SetAttribute("version", "1.0")

	if !s.IsAuthenticated() {
		// attach TLS feature
		tlsEnabled := s.cfg.TLS != nil
		tlsRequired := s.cfg.TLS != nil && s.cfg.TLS.Required

		if !s.IsSecured() && tlsEnabled {
			startTLS := xml.NewElementName("starttls")
			startTLS.SetNamespace("urn:ietf:params:xml:ns:xmpp-tls")
			if tlsRequired {
				startTLS.AppendElement(xml.NewElementName("required"))
			}
			features.AppendElement(startTLS)
		}

		// attach SASL mechanisms
		shouldOfferSASL := !tlsEnabled || (!tlsRequired || (tlsRequired && s.IsSecured()))

		if shouldOfferSASL && len(s.authrs) > 0 {
			mechanisms := xml.NewElementName("mechanisms")
			mechanisms.SetNamespace(saslNamespace)
			for _, athr := range s.authrs {
				// don't offset authenticators with channel binding on an unsecure stream
				if athr.UsesChannelBinding() && !s.IsSecured() {
					continue
				}
				mechanism := xml.NewElementName("mechanism")
				mechanism.SetText(athr.Mechanism())
				mechanisms.AppendElement(mechanism)
			}
			features.AppendElement(mechanisms)
		}

		// allow In-band registration over encrypted stream only
		allowRegistration := !tlsEnabled || (tlsEnabled && s.IsSecured())

		if _, ok := s.cfg.Modules["offline"]; ok && allowRegistration {
			registerFeature := xml.NewElementNamespace("register", "http://jabber.org/features/iq-register")
			features.AppendElement(registerFeature)
		}
		s.state = connected

	} else {
		// attach compression feature
		if !s.IsCompressed() && s.cfg.Compression != nil {
			compression := xml.NewElementNamespace("compression", "http://jabber.org/features/compress")
			method := xml.NewElementName("method")
			method.SetText("zlib")
			compression.AppendElement(method)
			features.AppendElement(compression)
		}
		session := xml.NewElementNamespace("session", "urn:ietf:params:xml:ns:xmpp-session")
		features.AppendElement(session)

		bind := xml.NewElementNamespace("bind", "urn:ietf:params:xml:ns:xmpp-bind")
		features.AppendElement(bind)

		s.state = authenticated
	}
	s.writeElement(features)
}

func (s *serverStream) handleConnected(elem xml.Element) {
	switch elem.Name() {
	case "starttls":
		if len(elem.Namespace()) > 0 && elem.Namespace() != tlsNamespace {
			s.disconnectWithStreamError(streamerror.ErrInvalidNamespace)
			return
		}
		s.proceedStartTLS()

	case "auth":
		if elem.Namespace() != saslNamespace {
			s.disconnectWithStreamError(streamerror.ErrInvalidNamespace)
			return
		}
		s.startAuthentication(elem)

	case "iq":
		stanza, _, err := s.buildStanza(elem)
		if err != nil {
			s.handleElementError(elem, err)
			return
		}
		iq := stanza.(*xml.IQ)

		if s.register != nil && s.register.MatchesIQ(iq) {
			s.register.ProcessIQ(iq)
			return

		} else if iq.FindElementNamespace("query", "jabber:iq:auth") != nil {
			// don't allow non-SASL authentication
			s.writeElement(iq.ServiceUnavailableError())
			return
		}
		fallthrough

	case "message", "presence":
		s.disconnectWithStreamError(streamerror.ErrNotAuthorized)

	default:
		s.disconnectWithStreamError(streamerror.ErrUnsupportedStanzaType)
	}
}

func (s *serverStream) handleAuthenticating(elem xml.Element) {
	if elem.Namespace() != saslNamespace {
		s.disconnectWithStreamError(streamerror.ErrInvalidNamespace)
		return
	}
	authr := s.activeAuthr
	s.continueAuthentication(elem, authr)
	if authr.Authenticated() {
		s.finishAuthentication(authr.Username())
	}
}

func (s *serverStream) handleAuthenticated(elem xml.Element) {
	switch elem.Name() {
	case "compress":
		if elem.Namespace() != compressProtocolNamespace {
			s.disconnectWithStreamError(streamerror.ErrUnsupportedStanzaType)
			return
		}
		s.compress(elem)

	case "iq":
		stanza, _, err := s.buildStanza(elem)
		if err != nil {
			s.handleElementError(elem, err)
			return
		}
		iq := stanza.(*xml.IQ)

		if len(s.Resource()) == 0 { // expecting bind
			s.bindResource(iq)
		} else { // expecting session
			s.startSession(iq)
		}

	default:
		s.disconnectWithStreamError(streamerror.ErrUnsupportedStanzaType)
	}
}

func (s *serverStream) handleSessionStarted(elem xml.Element) {
	// reset ping timer deadline
	if s.ping != nil {
		s.ping.ResetDeadline()
	}

	stanza, toJID, err := s.buildStanza(elem)
	if err != nil {
		s.handleElementError(elem, err)
		return
	}
	if s.isComponentDomain(toJID.Domain()) {
		s.processComponentStanza(stanza)
	} else {
		s.processStanza(stanza)
	}
}

func (s *serverStream) proceedStartTLS() {
	if s.IsSecured() {
		s.disconnectWithStreamError(streamerror.ErrNotAuthorized)
		return
	}
	cer, err := tls.LoadX509KeyPair(s.cfg.TLS.CertFile, s.cfg.TLS.PrivKeyFile)
	if err != nil {
		log.Error(err)
		s.writeElement(xml.NewElementNamespace("failure", tlsNamespace))
		s.disconnect(true)
		return
	}
	s.writeElement(xml.NewElementNamespace("proceed", tlsNamespace))

	cfg := &tls.Config{
		ServerName:   s.Domain(),
		Certificates: []tls.Certificate{cer},
	}
	s.tr.StartTLS(cfg)

	s.lock.Lock()
	s.secured = true
	s.lock.Unlock()

	log.Infof("secured stream... id: %s", s.id)

	s.restart()
}

func (s *serverStream) compress(elem xml.Element) {
	if s.IsCompressed() {
		s.disconnectWithStreamError(streamerror.ErrUnsupportedStanzaType)
		return
	}
	method := elem.FindElement("method")
	if method == nil || method.TextLen() == 0 {
		failure := xml.NewElementNamespace("failure", compressProtocolNamespace)
		failure.AppendElement(xml.NewElementName("setup-failed"))
		s.writeElement(failure)
		return
	}
	if method.Text() != "zlib" {
		failure := xml.NewElementNamespace("failure", compressProtocolNamespace)
		failure.AppendElement(xml.NewElementName("unsupported-method"))
		s.writeElement(failure)
		return
	}
	compressed := xml.NewElementNamespace("compressed", compressProtocolNamespace)
	s.writeElement(compressed)

	s.tr.EnableCompression(s.cfg.Compression.Level)
	s.lock.Lock()
	s.compressed = true
	s.lock.Unlock()

	log.Infof("compressed stream... id: %s", s.id)

	s.restart()
}

func (s *serverStream) startAuthentication(elem xml.Element) {
	mechanism := elem.Attribute("mechanism")
	for _, authr := range s.authrs {
		if authr.Mechanism() == mechanism {
			if err := s.continueAuthentication(elem, authr); err != nil {
				return
			}
			if authr.Authenticated() {
				s.finishAuthentication(authr.Username())
			} else {
				s.activeAuthr = authr
				s.state = authenticating
			}
			return
		}
	}

	// ...mechanism not found...
	failure := xml.NewElementNamespace("failure", saslNamespace)
	failure.AppendElement(xml.NewElementName("invalid-mechanism"))
	s.writeElement(failure)
}

func (s *serverStream) continueAuthentication(elem xml.Element, authr authenticator) error {
	err := authr.ProcessElement(elem)
	if saslErr, ok := err.(saslError); ok {
		s.failAuthentication(saslErr.Element())
	} else if err != nil {
		log.Error(err)
		s.failAuthentication(errSASLTemporaryAuthFailure.(saslError).Element())
	}
	return err
}

func (s *serverStream) finishAuthentication(username string) {
	if s.activeAuthr != nil {
		s.activeAuthr.Reset()
		s.activeAuthr = nil
	}
	s.lock.Lock()
	s.username = username
	s.authenticated = true
	s.jid, _ = xml.NewJID(s.username, s.domain, "", true)
	s.lock.Unlock()

	stream.C2S().AuthenticateStream(s)
	s.restart()
}

func (s *serverStream) failAuthentication(elem xml.Element) {
	failure := xml.NewElementNamespace("failure", saslNamespace)
	failure.AppendElement(elem)
	s.writeElement(failure)

	if s.activeAuthr != nil {
		s.activeAuthr.Reset()
		s.activeAuthr = nil
	}
	s.state = connected
}

func (s *serverStream) bindResource(iq *xml.IQ) {
	bind := iq.FindElementNamespace("bind", bindNamespace)
	if bind == nil {
		s.writeElement(iq.NotAllowedError())
		return
	}
	var resource string
	if resourceElem := bind.FindElement("resource"); resourceElem != nil {
		resource = resourceElem.Text()
	} else {
		resource = uuid.New()
	}
	// try binding...
	if !s.isResourceAvailable(resource) {
		s.writeElement(iq.ConflictError())
		return
	}
	userJID, err := xml.NewJID(s.Username(), s.Domain(), resource, false)
	if err != nil {
		s.writeElement(iq.BadRequestError())
		return
	}
	s.lock.Lock()
	s.resource = resource
	s.jid = userJID
	s.lock.Unlock()

	log.Infof("binded resource %s... (%s)", s.Resource(), s.Username())

	//...notify successful binding
	result := xml.NewIQType(iq.ID(), xml.ResultType)

	binded := xml.NewElementNamespace("bind", bindNamespace)
	jid := xml.NewElementName("jid")
	jid.SetText(s.Username() + "@" + s.Domain() + "/" + s.Resource())
	binded.AppendElement(jid)
	result.AppendElement(binded)

	s.writeElement(result)
}

func (s *serverStream) startSession(iq *xml.IQ) {
	sess := iq.FindElementNamespace("session", sessionNamespace)
	if sess == nil {
		s.writeElement(iq.NotAllowedError())
		return
	}
	s.writeElement(iq.ResultIQ())

	if s.ping != nil {
		s.ping.StartPinging()
	}
	s.state = sessionStarted
}

func (s *serverStream) processStanza(element xml.Element) {
	switch stanza := element.(type) {
	case *xml.IQ:
		s.processIQ(stanza)
	case *xml.Presence:
		s.processPresence(stanza)
	case *xml.Message:
		s.processMessage(stanza)
	}
}

func (s *serverStream) processComponentStanza(element xml.Element) {
}

func (s *serverStream) processIQ(iq *xml.IQ) {
	if !stream.C2S().IsLocalDomain(iq.ToJID().Domain()) {
		// TODO(ortuman): Implement XMPP federation
		return
	}

	toJid := iq.ToJID()
	if toJid.IsFull() {
		if err := s.sendElement(iq, toJid); err == errResourceNotFound {
			resp := iq.Copy()
			resp.SetFrom(toJid.String())
			resp.SetTo(s.JID().String())
			s.SendElement(resp.ServiceUnavailableError())
		}
		return
	}

	for _, handler := range s.iqHandlers {
		if !handler.MatchesIQ(iq) {
			continue
		}
		handler.ProcessIQ(iq)
		return
	}

	// ...IQ not handled...
	if iq.IsGet() || iq.IsSet() {
		s.writeElement(iq.ServiceUnavailableError())
	}
}

func (s *serverStream) processPresence(presence *xml.Presence) {
	if !stream.C2S().IsLocalDomain(presence.ToJID().Domain()) {
		// TODO(ortuman): Implement XMPP federation
		return
	}
	toJid := presence.ToJID()
	if toJid.IsBare() && (toJid.Node() != s.Username() || toJid.Domain() != s.Domain()) {
		if s.roster != nil {
			s.roster.ProcessPresence(presence)
		}
		return
	}
	if toJid.IsFull() {
		s.sendElement(presence, toJid)
		return
	}

	// set resource priority & availability
	s.lock.Lock()
	s.priority = presence.Priority()
	switch presence.Type() {
	case xml.AvailableType:
		s.available = true
		s.presenceElements = presence.Elements()
	case xml.UnavailableType:
		s.available = false
		s.presenceElements = nil
	}
	s.lock.Unlock()

	// deliver pending approval notifications
	if s.roster != nil {
		s.rosterOnce.Do(func() {
			s.roster.DeliverPendingApprovalNotifications()
			s.roster.ReceivePresences()
		})
		s.roster.BroadcastPresence(presence)
	}

	// deliver offline messages
	if s.offline != nil && s.Priority() >= 0 {
		s.offlineOnce.Do(func() {
			s.offline.DeliverOfflineMessages()
		})
	}
}

func (s *serverStream) processMessage(message *xml.Message) {
	if !stream.C2S().IsLocalDomain(message.ToJID().Domain()) {
		// TODO(ortuman): Implement XMPP federation
		return
	}

	err := s.sendElement(message, message.ToJID())
	switch err {
	case errNotAuthenticated:
		if s.offline != nil {
			s.offline.ArchiveMessage(message)
		}
	case errResourceNotFound:
		resp := message.Copy()
		resp.SetFrom(message.ToJID().String())
		resp.SetTo(s.JID().String())
		s.SendElement(resp.ServiceUnavailableError())
	}
}

func (s *serverStream) restart() {
	s.state = connecting
	s.parser = xml.NewParser(s.tr)
}

func (s *serverStream) loop() {
	s.doRead() // start reading transport...
	for {
		// stop looping after disconnecting stream
		if s.state == disconnected {
			return
		}

		select {
		case e := <-s.writeCh:
			s.writeElement(e)

		case e := <-s.readCh:
			s.handleElement(e)
			if s.state != disconnected {
				s.doRead() // keep reading transport...
			}

		case err := <-s.discCh:
			switch err {
			case nil:
				s.disconnect(false)
			default:
				if strmErr, ok := err.(*streamerror.Error); ok {
					s.disconnectWithStreamError(strmErr)
				} else {
					log.Error(err)
					s.disconnect(false)
				}
			}
		}
	}
}

func (s *serverStream) doRead() {
	go func() {
		if e, err := s.parser.ParseElement(); e != nil && err == nil {
			log.Debugf("RECV: %v", e)
			s.readCh <- e

		} else if err != nil {
			switch err {
			case nil:
				break
			case io.EOF, io.ErrUnexpectedEOF, xml.ErrStreamClosedByPeer:
				s.discCh <- nil
			default:
				log.Error(err)
				s.discCh <- streamerror.ErrInvalidXML
			}
		}
	}()
}

func (s *serverStream) openStreamElement() {
	ops := xml.NewElementName("stream:stream")
	ops.SetAttribute("xmlns", s.streamDefaultNamespace())
	ops.SetAttribute("xmlns:stream", streamNamespace)
	ops.SetAttribute("id", uuid.New())
	ops.SetAttribute("from", s.Domain())
	ops.SetAttribute("version", "1.0")

	s.tr.Write([]byte(`<?xml version="1.0"?>`))
	ops.ToXML(s.tr, false)
}

func (s *serverStream) buildStanza(elem xml.Element) (xml.Element, *xml.JID, error) {
	if err := s.validateNamespace(elem); err != nil {
		return nil, nil, err
	}
	fromJID, toJID, err := s.validateAddresses(elem)
	if err != nil {
		return nil, nil, err
	}
	switch elem.Name() {
	case "iq":
		iq, err := xml.NewIQFromElement(elem, fromJID, toJID)
		if err != nil {
			log.Error(err)
			return nil, nil, xml.ErrBadRequest
		}
		return iq, iq.ToJID(), nil

	case "presence":
		presence, err := xml.NewPresenceFromElement(elem, fromJID, toJID)
		if err != nil {
			log.Error(err)
			return nil, nil, xml.ErrBadRequest
		}
		return presence, presence.ToJID(), nil

	case "message":
		message, err := xml.NewMessageFromElement(elem, fromJID, toJID)
		if err != nil {
			log.Error(err)
			return nil, nil, xml.ErrBadRequest
		}
		return message, message.ToJID(), nil
	}
	return nil, nil, streamerror.ErrUnsupportedStanzaType
}

func (s *serverStream) handleElementError(elem xml.Element, err error) {
	if streamErr, ok := err.(*streamerror.Error); ok {
		s.disconnectWithStreamError(streamErr)
	} else if stanzaErr, ok := err.(*xml.StanzaError); ok {
		s.writeElement(xml.ToErrorElement(elem, stanzaErr))
	} else {
		log.Error(err)
	}
}

func (s *serverStream) validateStreamElement(elem xml.Element) *streamerror.Error {
	if elem.Name() != "stream:stream" {
		return streamerror.ErrUnsupportedStanzaType
	}
	to := elem.To()
	if len(to) > 0 && !stream.C2S().IsLocalDomain(to) {
		return streamerror.ErrHostUnknown
	}
	if elem.Namespace() != s.streamDefaultNamespace() || elem.Attribute("xmlns:stream") != streamNamespace {
		return streamerror.ErrInvalidNamespace
	}
	if elem.Version() != "1.0" {
		return streamerror.ErrUnsupportedVersion
	}
	return nil
}

func (s *serverStream) validateNamespace(elem xml.Element) *streamerror.Error {
	ns := elem.Namespace()
	if len(ns) == 0 || ns == s.streamDefaultNamespace() {
		return nil
	}
	return streamerror.ErrInvalidNamespace
}

func (s *serverStream) validateAddresses(elem xml.Element) (fromJID *xml.JID, toJID *xml.JID, err error) {
	// validate from JID
	from := elem.From()
	if len(from) > 0 && !s.isValidFrom(from) {
		return nil, nil, streamerror.ErrInvalidFrom
	}
	fromJID = s.JID()

	// validate to JID
	to := elem.To()
	if len(to) > 0 {
		toJID, err = xml.NewJIDString(elem.To(), false)
		if err != nil {
			return nil, nil, xml.ErrJidMalformed
		}
	} else {
		toJID, err = xml.NewJID("", s.Domain(), "", true)
	}
	return
}

func (s *serverStream) isValidFrom(from string) bool {
	validFrom := false
	j, err := xml.NewJIDString(from, false)
	if err == nil && j != nil {
		node := j.Node()
		domain := j.Domain()
		resource := j.Resource()

		userJID := s.JID()
		validFrom = node == userJID.Node() && domain == userJID.Domain()
		if len(resource) > 0 {
			validFrom = validFrom && resource == userJID.Resource()
		}
	}
	return validFrom
}

func (s *serverStream) isComponentDomain(domain string) bool {
	return false
}

func (s *serverStream) streamDefaultNamespace() string {
	switch s.cfg.Type {
	case config.C2SServerType:
		return jabberClientNamespace
	case config.S2SServerType:
		return jabberServerNamespace
	}
	// should not be reached
	return ""
}

func (s *serverStream) writeElement(elem xml.Element) {
	log.Debugf("SEND: %v", elem)
	elem.ToXML(s.tr, true)
}

func (s *serverStream) disconnectWithStreamError(err *streamerror.Error) {
	if s.state == connecting {
		s.openStreamElement()
	}
	s.writeElement(err.Element())
	s.disconnect(true)
}

func (s *serverStream) disconnect(closeStream bool) {
	s.lock.RLock()
	available := s.available
	s.lock.RUnlock()

	if available && s.roster != nil {
		s.roster.BroadcastPresence(xml.NewPresence(s.JID(), s.JID(), xml.UnavailableType))
	}
	if closeStream {
		s.tr.Write([]byte("</stream:stream>"))
	}
	s.tr.Close()

	s.state = disconnected

	stream.C2S().UnregisterStream(s)
}

func (s *serverStream) isResourceAvailable(resource string) bool {
	strms := stream.C2S().AvailableStreams(s.Username())
	for _, strm := range strms {
		if strm.Resource() == resource {
			return false
		}
	}
	return true
}

func (s *serverStream) sendElement(serializable xml.Element, to *xml.JID) error {
	recipients := stream.C2S().AvailableStreams(to.Node())
	if len(recipients) == 0 {
		return errNotAuthenticated
	}
	if to.IsFull() {
		for _, strm := range recipients {
			if strm.Resource() == to.Resource() {
				strm.SendElement(serializable)
				return nil
			}
		}
		return errResourceNotFound

	} else {
		switch serializable.(type) {
		case *xml.Message:
			// send to highest priority stream
			strm := recipients[0]
			highestPriority := strm.Priority()
			for i := 1; i < len(recipients); i++ {
				if recipients[i].Priority() > highestPriority {
					strm = recipients[i]
				}
			}
			strm.SendElement(serializable)

		default:
			// broadcast to all streams
			for _, strm := range recipients {
				strm.SendElement(serializable)
			}
		}
	}
	return nil
}
