/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package module

import "github.com/ortuman/jackal/xml"

type Module interface {
	AssociatedNamespaces() []string
}

type IQHandler interface {
	Module
	MatchesIQ(*xml.IQ) bool
	ProcessIQ(*xml.IQ)
}

type Stream interface {
	Username() string
	Domain() string
	Resource() string

	JID() *xml.JID

	Secured() bool
	Authenticated() bool

	SendElement(element xml.Serializable)
	RosterPush(query *xml.Element)

	Disconnect(err error)
}
