/*
 * Copyright (c) 2018 Miguel Ángel Ortuño.
 * See the LICENSE file for more information.
 */

package xml

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	AvailableType    = ""
	UnavailableType  = "unavailable"
	SubscribeType    = "subscribe"
	UnsubscribeType  = "unsubscribe"
	SubscribedType   = "subscribed"
	UnsubscribedType = "unsubscribed"
)

type ShowState int

const (
	AvailableShowState ShowState = iota
	AwayShowState
	ChatShowState
	DoNotDisturbShowState
	ExtendedAwaysShowState
)

type Presence struct {
	XElement
	to        *JID
	from      *JID
	showState ShowState
	priority  int8
}

func NewPresenceFromElement(e Element, from *JID, to *JID) (*Presence, error) {
	if e.Name() != "presence" {
		return nil, fmt.Errorf("wrong Presence element name: %s", e.Name())
	}
	presenceType := e.Attribute("type")
	if !isPresenceType(presenceType) {
		return nil, fmt.Errorf(`invalid Presence "type" attribute: %s`, presenceType)
	}
	p := &Presence{}
	p.SetName(e.Name())
	p.attrs = e.Attributes()
	p.elements = e.Elements()

	// show
	if err := p.setShow(); err != nil {
		return nil, err
	}
	// status
	if err := p.validateStatus(); err != nil {
		return nil, err
	}
	// priority
	if err := p.setPriority(); err != nil {
		return nil, err
	}
	p.SetAttribute("to", to.ToFullJID())
	p.SetAttribute("from", from.ToFullJID())
	p.to = to
	p.from = from
	return p, nil
}

func NewPresence(from string, to string, presenceType string) *Presence {
	p := &Presence{}
	p.SetName("presence")
	p.SetFrom(from)
	p.SetTo(to)
	p.SetType(presenceType)
	return p
}

// IsAvailable returns true if this is an 'available' type Presence.
func (p *Presence) IsAvailable() bool {
	return p.Type() == AvailableType
}

// IsUnavailable returns true if this is an 'unavailable' type Presence.
func (p *Presence) IsUnavailable() bool {
	return p.Type() == UnavailableType
}

// IsSubscribe returns true if this is a 'subscribe' type Presence.
func (p *Presence) IsSubscribe() bool {
	return p.Type() == SubscribeType
}

// IsUnsubscribe returns true if this is an 'unsubscribe' type Presence.
func (p *Presence) IsUnsubscribe() bool {
	return p.Type() == UnsubscribeType
}

// IsSubscribed returns true if this is a 'subscribed' type Presence.
func (p *Presence) IsSubscribed() bool {
	return p.Type() == SubscribedType
}

// IsUnsubscribed returns true if this is an 'unsubscribed' type Presence.
func (p *Presence) IsUnsubscribed() bool {
	return p.Type() == UnsubscribedType
}

// ShowState returns presence stanza show state.
func (p *Presence) ShowState() ShowState {
	return p.showState
}

// Priority returns presence stanza priority value.
func (p *Presence) Priority() int8 {
	return p.priority
}

// ToJID satisfies stanza interface.
func (p *Presence) ToJID() *JID {
	return p.to
}

// FromJID satisfies stanza interface.
func (p *Presence) FromJID() *JID {
	return p.from
}

func isPresenceType(presenceType string) bool {
	switch presenceType {
	case AvailableType, UnavailableType, SubscribeType, UnsubscribeType, SubscribedType, UnsubscribedType:
		return true
	default:
		return false
	}
}

func (p *Presence) validateStatus() error {
	sts := p.FindElements("status")
	for _, st := range sts {
		switch st.AttributesCount() {
		case 0:
			break
		case 1:
			if st.Attributes()[0].Label == "xml:lang" {
				break
			}
			fallthrough
		default:
			return errors.New(" the <status/> element MUST NOT possess any attributes, with the exception of the 'xml:lang' attribute")
		}
	}
	return nil
}

func (p *Presence) setShow() error {
	shs := p.FindElements("show")
	switch len(shs) {
	case 0:
		p.showState = AvailableShowState
	case 1:
		if shs[0].AttributesCount() > 0 {
			return errors.New(" the <show/> element MUST NOT possess any attributes")
		}
		switch shs[0].Text() {
		case "away":
			p.showState = AwayShowState
		case "chat":
			p.showState = ChatShowState
		case "dnd":
			p.showState = DoNotDisturbShowState
		case "xa":
			p.showState = ExtendedAwaysShowState
		default:
			return fmt.Errorf("invalid Presence show state: %s", shs[0].Text())
		}

	default:
		return errors.New(" Presence stanza MUST NOT contain more than one <show/> element")
	}
	return nil
}

func (p *Presence) setPriority() error {
	ps := p.FindElements("priority")
	switch len(ps) {
	case 0:
		break
	case 1:
		pr, err := strconv.Atoi(ps[0].Text())
		if err != nil {
			return err
		}
		if pr < -128 || pr > 127 {
			return errors.New("priority value MUST be an integer between -128 and +127")
		}
		p.priority = int8(pr)

	default:
		return errors.New("a Presence stanza MUST NOT contain more than one <priority/> element")
	}
	return nil
}
