/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package xml

// Serializable is an interface type. A Serializable entity describes a value
// that could be serialized to a raw XML representation.
// includeClosing determines if closing tag should be attached.
type Serializable interface {
	XML(includeClosing bool) string
}

// Attribute represents an XML node attribute (label=value).
type Attribute struct {
	label string
	value string
}

// Element represents an immutable XML node element.
type Element struct {
	name     string
	text     string
	attrs    []Attribute
	elements []*Element
}

// NewElementName creates an XML Element instance with a given name.
func NewElementName(name string) *Element {
	e := Element{}
	e.name = name
	e.attrs = []Attribute{}
	e.elements = []*Element{}
	return &e
}

// NewElementAttributes creates an XML Element instance with a given name and attributes.
func NewElementAttributes(name string, attributes []Attribute) *Element {
	e := Element{}
	e.name = name
	e.attrs = attributes
	e.elements = []*Element{}
	return &e
}

// NewElementNamespace creates an XML Element instance with a given name and namespace.
func NewElementNamespace(name, namespace string) *Element {
	return NewElementAttributes(name, []Attribute{{"xmlns", namespace}})
}

// Name returns XML node name.
func (e *Element) Name() string {
	return e.name
}

// Text returns XML node text value.
// Returns an empty string if not set.
func (e *Element) Text() string {
	return e.text
}

// TextLen returns XML node text value length.
func (e *Element) TextLen() int {
	return len(e.text)
}

// Attribute returns XML node attribute value.
func (e *Element) Attribute(label string) string {
	for i := 0; i < len(e.attrs); i++ {
		if e.attrs[i].label == label {
			return e.attrs[i].value
		}
	}
	return ""
}

// AttributesCount XML attributes count.
func (e *Element) AttributesCount() int {
	return len(e.attrs)
}

// FindElement returns first element identified by name.
// Returns nil if no element is found.
func (e *Element) FindElement(name string) *Element {
	for i := 0; i < len(e.elements); i++ {
		if e.elements[i].name == name {
			return e.elements[i]
		}
	}
	return nil
}

// FindElements returns all elements identified by name.
// Returns an empty array if no elements are found.
func (e *Element) FindElements(name string) []*Element {
	ret := e.elements[:0]
	for i := 0; i < len(e.elements); i++ {
		if e.elements[i].name == name {
			ret = append(ret, e.elements[i])
		}
	}
	return ret
}

// FindElementNamespace returns first element identified by name and namespace.
// Returns nil if no element is found.
func (e *Element) FindElementNamespace(name, namespace string) *Element {
	for i := 0; i < len(e.elements); i++ {
		if e.elements[i].name == name && e.elements[i].Namespace() == namespace {
			return e.elements[i]
		}
	}
	return nil
}

// FindElementsNamespace returns all elements identified by name and namespace.
// Returns an empty array if no elements are found.
func (e *Element) FindElementsNamespace(name, namespace string) []*Element {
	ret := e.elements[:0]
	for i := 0; i < len(e.elements); i++ {
		if e.elements[i].name == name && e.elements[i].Namespace() == namespace {
			ret = append(ret, e.elements[i])
		}
	}
	return ret
}

// Elements returns all instance's child elements.
func (e *Element) Elements() []*Element {
	return e.elements
}

// ElementsCount returns child elements count.
func (e *Element) ElementsCount() int {
	return len(e.elements)
}

// Copy returns a deep copy of this element.
func (e *Element) Copy() *Element {
	cp := &Element{}
	cp.name = e.name
	cp.text = e.text
	cp.attrs = make([]Attribute, len(e.attrs), cap(e.attrs))
	cp.elements = make([]*Element, len(e.elements), cap(e.elements))
	copy(cp.attrs, e.attrs)
	copy(cp.elements, e.elements)
	return cp
}

// IsError returns true if element has a 'type' attribute of value 'error'.
func (e *Element) IsError() bool {
	return e.Type() == "error"
}

// String returns a string representation of the element.
func (e *Element) String() string {
	return e.XML(true)
}

// XML satisfies Serializable interface.
func (e *Element) XML(includeClosing bool) string {
	ret := "<" + e.name

	// serialize attributes
	for i := 0; i < len(e.attrs); i++ {
		if len(e.attrs[i].value) == 0 {
			continue
		}
		ret += " " + e.attrs[i].label + "=\"" + e.attrs[i].value + "\""
	}
	if len(e.elements) > 0 || len(e.text) > 0 {
		ret += ">"

		// serialize text
		if len(e.text) > 0 {
			ret += e.text
		}
		// serialize child elements
		for j := 0; j < len(e.elements); j++ {
			ret += e.elements[j].XML(true)
		}
		if includeClosing {
			ret += "</" + e.name + ">"
		}
	} else {
		if includeClosing {
			ret += "/>"
		} else {
			ret += ">"
		}
	}
	return ret
}

func (e *Element) copyAttributes(attribs []Attribute) {
	e.attrs = make([]Attribute, len(attribs), cap(attribs))
	copy(e.attrs, attribs)
}

func (e *Element) copyElements(elements []*Element) {
	e.elements = make([]*Element, len(elements), cap(elements))
	copy(e.elements, elements)
}

func (e *Element) setAttribute(label, value string) {
	for i := 0; i < len(e.attrs); i++ {
		if e.attrs[i].label == label {
			e.attrs[i].value = value
			return
		}
	}
	e.attrs = append(e.attrs, Attribute{label, value})
}

func (e *Element) removeAttribute(label string) {
	for i := 0; i < len(e.attrs); i++ {
		if e.attrs[i].label == label {
			e.attrs = append(e.attrs[:i], e.attrs[i+1:]...)
			return
		}
	}
}

func (e *Element) appendElement(element *Element) {
	e.elements = append(e.elements, element)
}

func (e *Element) appendElements(elements []*Element) {
	e.elements = append(e.elements, elements...)
}

func (e *Element) removeElements(name string) {
	filtered := e.elements[:0]
	for _, elem := range e.elements {
		if elem.name != name {
			filtered = append(filtered, elem)
		}
	}
	e.elements = filtered
}

func (e *Element) removeElementsNamespace(name, namespace string) {
	filtered := e.elements[:0]
	for _, elem := range e.elements {
		if elem.name != name || elem.Namespace() != namespace {
			filtered = append(filtered, elem)
		}
	}
	e.elements = filtered
}

func (e *Element) clearElements() {
	e.elements = []*Element{}
}
