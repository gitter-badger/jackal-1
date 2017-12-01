/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package xml

import (
	"errors"
	"fmt"
)

const (
	getIQType    = "get"
	setIQType    = "set"
	resultIQType = "result"
)

type IQ struct {
	Element
	to   *JID
	from *JID
}

func NewIQ(e *Element, to *JID, from *JID) (*IQ, error) {
	if e.name != "iq" {
		return nil, fmt.Errorf("wrong iq element name: %s", e.name)
	}
	if e.TextLen() != 0 {
		return nil, errors.New("iq bad format")
	}
	if !isIQType(e.Type()) {
		return nil, fmt.Errorf("wrong iq type: %s", e.Type())
	}
	iq := &IQ{}
	iq.name = e.name
	iq.copyAttributes(e.attrs)
	iq.copyElements(e.elements)
	iq.to = to
	iq.from = from
	return iq, nil
}

// IsGet returns true if this is a 'get' type IQ.
func (iq *IQ) IsGet() bool {
	return iq.Type() == getIQType
}

// IsSet returns true if this is a 'set' type IQ.
func (iq *IQ) IsSet() bool {
	return iq.Type() == setIQType
}

// IsResult returns true if this is a 'result' type IQ.
func (iq *IQ) IsResult() bool {
	return iq.Type() == resultIQType
}

// ResultIQ returns the instance associated result IQ.
func (iq *IQ) ResultIQ(from string) *IQ {
	rs := &IQ{}
	rs.name = "iq"
	rs.setAttribute("type", resultIQType)
	rs.setAttribute("id", iq.ID())
	rs.setAttribute("to", iq.From())
	if len(from) > 0 {
		rs.setAttribute("from", iq.From())
	}
	return rs
}

func isIQType(tp string) bool {
	switch tp {
	case getIQType, setIQType, resultIQType, "error":
		return true
	}
	return false
}
