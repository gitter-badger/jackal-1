/*
 * Copyright (c) 2018 Miguel Ángel Ortuño.
 * See the LICENSE file for more information.
 */

package module

import (
	"sort"

	"github.com/ortuman/jackal/stream"
	"github.com/ortuman/jackal/xml"
)

const (
	discoInfoNamespace  = "http://jabber.org/protocol/disco#info"
	discoItemsNamespace = "http://jabber.org/protocol/disco#items"
)

type DiscoItem struct {
	Jid  string
	Name string
	Node string
}

type DiscoIdentity struct {
	Category string
	Type     string
	Name     string
}

type XEPDiscoInfo struct {
	strm       stream.C2SStream
	identities []DiscoIdentity
	features   []string
	items      []DiscoItem
}

func NewXEPDiscoInfo(strm stream.C2SStream) *XEPDiscoInfo {
	x := &XEPDiscoInfo{
		strm: strm,
	}
	return x
}

func (x *XEPDiscoInfo) Identities() []DiscoIdentity {
	return x.identities
}

func (x *XEPDiscoInfo) SetIdentities(identities []DiscoIdentity) {
	x.identities = identities
}

func (x *XEPDiscoInfo) Features() []string {
	return x.features
}

func (x *XEPDiscoInfo) SetFeatures(features []string) {
	x.features = features
}

func (x *XEPDiscoInfo) Items() []DiscoItem {
	return x.items
}

func (x *XEPDiscoInfo) SetItems(items []DiscoItem) {
	x.items = items
}

func (x *XEPDiscoInfo) AssociatedNamespaces() []string {
	return []string{discoInfoNamespace, discoItemsNamespace}
}

func (x *XEPDiscoInfo) MatchesIQ(iq *xml.IQ) bool {
	q := iq.FindElement("query")
	if q == nil {
		return false
	}
	return iq.IsGet() && (q.Namespace() == discoInfoNamespace || q.Namespace() == discoItemsNamespace)
}

func (x *XEPDiscoInfo) ProcessIQ(iq *xml.IQ) {
	if !iq.ToJID().IsServer() {
		x.strm.SendElement(iq.FeatureNotImplementedError())
		return
	}
	q := iq.FindElement("query")
	switch q.Namespace() {
	case discoInfoNamespace:
		x.sendDiscoInfo(iq)
	case discoItemsNamespace:
		x.sendDiscoItems(iq)
	}
}

func (x *XEPDiscoInfo) sendDiscoInfo(iq *xml.IQ) {
	sort.Slice(x.features, func(i, j int) bool { return x.features[i] < x.features[j] })

	result := iq.ResultIQ()
	query := xml.NewElementNamespace("query", discoInfoNamespace)

	for _, identity := range x.identities {
		identityEl := xml.NewElementName("identity")
		identityEl.SetAttribute("category", identity.Category)
		if len(identity.Type) > 0 {
			identityEl.SetAttribute("type", identity.Type)
		}
		if len(identity.Name) > 0 {
			identityEl.SetAttribute("name", identity.Name)
		}
		query.AppendElement(identityEl)
	}
	for _, feature := range x.features {
		featureEl := xml.NewElementName("feature")
		featureEl.SetAttribute("var", feature)
		query.AppendElement(featureEl)
	}

	result.AppendElement(query)
	x.strm.SendElement(result)
}

func (x *XEPDiscoInfo) sendDiscoItems(iq *xml.IQ) {
	result := iq.ResultIQ()
	query := xml.NewElementNamespace("query", discoItemsNamespace)

	for _, item := range x.items {
		itemEl := xml.NewElementName("item")
		itemEl.SetAttribute("jid", item.Jid)
		if len(item.Name) > 0 {
			itemEl.SetAttribute("name", item.Name)
		}
		if len(item.Node) > 0 {
			itemEl.SetAttribute("node", item.Node)
		}
		query.AppendElement(itemEl)
	}

	result.AppendElement(query)
	x.strm.SendElement(result)
}
