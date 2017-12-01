/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package xml

// MutableElement represents a mutable XML node element.
// This type adds mutable operations to the basic behavior inherited from Element.
type MutableElement struct {
	Element
}

// NewMutableElement creates a MutableElement instance from an immutable element.
func NewMutableElement(e *Element) *MutableElement {
	m := MutableElement{}
	m.name = e.name
	m.text = e.text
	m.copyAttributes(e.attrs)
	m.copyElements(e.elements)
	return &m
}

// NewMutableElementName creates MutableElement instance with a given name.
func NewMutableElementName(name string) *MutableElement {
	m := &MutableElement{}
	m.name = name
	m.attrs = []Attribute{}
	m.elements = []*Element{}
	return m
}

// NewMutableElementAttributes creates MutableElement instance with a given name and attributes.
func NewMutableElementAttributes(name string, attributes []Attribute) *MutableElement {
	m := &MutableElement{}
	m.name = name
	m.attrs = attributes
	m.elements = []*Element{}
	return m
}

// NewMutableElementNamespace creates MutableElement instance with a given name and namespace.
func NewMutableElementNamespace(name, namespace string) *MutableElement {
	return NewMutableElementAttributes(name, []Attribute{{"xmlns", namespace}})
}

// SetName sets XML node name.
func (m *MutableElement) SetName(name string) {
	m.name = name
}

// SetText sets XML node text value.
func (m *MutableElement) SetText(text string) {
	m.text = text
}

// SetAttribute sets an XML node attribute (label=value)
func (m *MutableElement) SetAttribute(label, value string) {
	m.setAttribute(label, value)
}

// RemoveAttribute removes an XML node attribute.
func (m *MutableElement) RemoveAttribute(label string) {
	m.removeAttribute(label)
}

// AppendElement appends a new subelement.
func (m *MutableElement) AppendElement(element *Element) {
	m.appendElement(element)
}

// AppendElements appends an array of elements.
func (m *MutableElement) AppendElements(elements []*Element) {
	m.appendElements(elements)
}

// RemoveElements removes all elements with a given name.
func (m *MutableElement) RemoveElements(name string) {
	m.removeElements(name)
}

// RemoveElementsNamespace removes all elements with a given name and namespace.
func (m *MutableElement) RemoveElementsNamespace(name, namespace string) {
	m.removeElementsNamespace(name, namespace)
}

// ClearElements removes all elements.
func (m *MutableElement) ClearElements() {
	m.clearElements()
}

// MutableCopy returns a new instance that’s an mutable copy of the receiver.
func (m *MutableElement) MutableCopy() *MutableElement {
	cp := &MutableElement{}
	cp.name = m.name
	cp.text = m.text
	cp.copyAttributes(m.attrs)
	cp.copyElements(m.elements)
	return cp
}
