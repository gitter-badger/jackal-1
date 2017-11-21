/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package xml

import (
	"encoding/xml"
	"fmt"
	"io"
)

const rootElementIndex = -1

type Parser struct {
	elements     []*Element
	parsingIndex int
	parsingStack []*MutableElement
	inElement    bool
}

func NewParser() *Parser {
	p := &Parser{}
	p.elements = make([]*Element, 0)
	p.parsingIndex = rootElementIndex
	p.parsingStack = make([]*MutableElement, 0)
	return p
}

func (p *Parser) ParseElements(reader io.Reader) error {
	d := xml.NewDecoder(reader)
	t, err := d.RawToken()
	for t != nil {
		switch t1 := t.(type) {
		case xml.StartElement:
			p.startElement(t1)
		case xml.CharData:
			p.setElementText(t1)
		case xml.EndElement:
			if err := p.endElement(t1); err != nil {
				return err
			}
		}
		t, err = d.RawToken()
	}
	if err != nil && err != io.EOF {
		return err
	}
	if p.parsingIndex == 0 && p.parsingStack[0].Name() == "stream:stream" {
		p.closeElement()
	}
	return nil
}

func (p *Parser) PopElement() *Element {
	if len(p.elements) == 0 {
		return nil
	}
	element := p.elements[0]
	p.elements = append(p.elements[:0], p.elements[1:]...)
	return element
}

func (p *Parser) startElement(t xml.StartElement) {
	var name string
	if len(t.Name.Space) > 0 {
		name = fmt.Sprintf("%s:%s", t.Name.Space, t.Name.Local)
	} else {
		name = t.Name.Local
	}

	attrs := []Attribute{}
	for _, a := range t.Attr {
		var label string
		if len(a.Name.Space) > 0 {
			label = fmt.Sprintf("%s:%s", a.Name.Space, a.Name.Local)
		} else {
			label = a.Name.Local
		}
		attrs = append(attrs, Attribute{label, a.Value})
	}
	element := NewMutableElementAttributes(name, attrs)
	p.parsingStack = append(p.parsingStack, element)
	p.parsingIndex++
	p.inElement = true
}

func (p *Parser) setElementText(t xml.CharData) {
	if !p.inElement {
		return
	}
	p.parsingStack[p.parsingIndex].SetText(string(t))
}

func (p *Parser) endElement(t xml.EndElement) error {
	var name string
	if len(t.Name.Space) > 0 {
		name = fmt.Sprintf("%s:%s", t.Name.Space, t.Name.Local)
	} else {
		name = t.Name.Local
	}
	if p.parsingStack[p.parsingIndex].Name() != name {
		return fmt.Errorf("unexpected end element </" + name + ">")
	}
	p.closeElement()
	return nil
}

func (p *Parser) closeElement() {
	element := p.parsingStack[p.parsingIndex]
	p.parsingStack = p.parsingStack[:p.parsingIndex]

	p.parsingIndex--
	if p.parsingIndex == rootElementIndex {
		p.elements = append(p.elements, element.Copy())
	} else {
		p.parsingStack[p.parsingIndex].AppendElement(element.Copy())
	}
	p.inElement = false
}
