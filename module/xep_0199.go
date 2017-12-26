/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package module

import (
	"sync"
	"time"

	"sync/atomic"

	"github.com/ortuman/jackal/config"
	"github.com/ortuman/jackal/errors"
	"github.com/ortuman/jackal/xml"
	"github.com/pborman/uuid"
)

const pingNamespace = "urn:xmpp:ping"

type XEPPing struct {
	cfg  *config.ModPing
	strm Stream

	pingTm *time.Timer
	pongCh chan struct{}

	pingMu sync.RWMutex // guards 'pingID'
	pingId string

	waitingPing uint32
	pingingOnce sync.Once
}

func NewXEPPing(cfg *config.ModPing, strm Stream) *XEPPing {
	return &XEPPing{
		cfg:    cfg,
		strm:   strm,
		pongCh: make(chan struct{}, 1),
	}
}

func (x *XEPPing) AssociatedNamespaces() []string {
	return []string{pingNamespace}
}

func (x *XEPPing) MatchesIQ(iq *xml.IQ) bool {
	return x.isPongIQ(iq) || iq.FindElementNamespace("ping", pingNamespace) != nil
}

func (x *XEPPing) ProcessIQ(iq *xml.IQ) {
	if x.isPongIQ(iq) {
		x.handlePongIQ(iq)
		return
	}
	toJid := iq.ToJID()
	if toJid.IsBare() && toJid.Node() != x.strm.Username() {
		x.strm.SendElement(iq.ForbiddenError())
		return
	}
	p := iq.FindElementNamespace("ping", pingNamespace)
	if p.ElementsCount() > 0 {
		x.strm.SendElement(iq.BadRequestError())
		return
	}
	if iq.IsGet() {
		x.strm.SendElement(iq.ResultIQ())
	} else {
		x.strm.SendElement(iq.BadRequestError())
	}
}

func (x *XEPPing) StartPinging() {
	if !x.cfg.Send {
		return
	}
	x.pingingOnce.Do(func() {
		x.pingTm = time.AfterFunc(time.Second*time.Duration(x.cfg.SendInterval), x.sendPing)
	})
}

func (x *XEPPing) ResetDeadline() {
	if !x.cfg.Send {
		return
	}
	if atomic.LoadUint32(&x.waitingPing) == 1 {
		x.pingTm.Reset(time.Second * time.Duration(x.cfg.SendInterval))
	}
}

func (x *XEPPing) isPongIQ(iq *xml.IQ) bool {
	x.pingMu.RLock()
	defer x.pingMu.RUnlock()
	return x.pingId == iq.ID() && (iq.IsResult() || iq.IsError())
}

func (x *XEPPing) sendPing() {
	atomic.StoreUint32(&x.waitingPing, 0)

	x.pingMu.Lock()
	x.pingId = uuid.New()
	pingId := x.pingId
	x.pingMu.Unlock()

	iq := xml.NewMutableIQType(pingId, xml.GetType)
	iq.SetTo(x.strm.JID().String())
	iq.AppendElement(xml.NewElementNamespace("ping", pingNamespace))

	x.strm.SendElement(iq)
	x.waitForPong()
}

func (x *XEPPing) waitForPong() {
	t := time.NewTimer(time.Second * time.Duration(x.cfg.SendInterval))
	select {
	case <-x.pongCh:
		return
	case <-t.C:
		x.strm.Disconnect(errors.ErrConnectionTimeout)
	}
}

func (x *XEPPing) handlePongIQ(iq *xml.IQ) {
	x.pingMu.Lock()
	x.pingId = ""
	x.pingMu.Unlock()

	x.pongCh <- struct{}{}
	x.pingTm.Reset(time.Second * time.Duration(x.cfg.SendInterval))
	atomic.StoreUint32(&x.waitingPing, 1)
}