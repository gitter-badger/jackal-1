/*
 * Copyright (c) 2018 Miguel Ángel Ortuño.
 * See the LICENSE file for more information.
 */

package module

import (
	"time"

	"github.com/ortuman/jackal/concurrent"
	"github.com/ortuman/jackal/config"
	"github.com/ortuman/jackal/log"
	"github.com/ortuman/jackal/storage"
	"github.com/ortuman/jackal/stream"
	"github.com/ortuman/jackal/xml"
)

type ModOffline struct {
	queue concurrent.OperationQueue
	cfg   *config.ModOffline
	strm  stream.C2SStream
}

func NewOffline(config *config.ModOffline, strm stream.C2SStream) *ModOffline {
	return &ModOffline{
		queue: concurrent.OperationQueue{
			QueueSize: 32,
			Timeout:   time.Second,
		},
		cfg:  config,
		strm: strm,
	}
}

func (o *ModOffline) AssociatedNamespaces() []string {
	return []string{"msgoffline"}
}

func (o *ModOffline) ArchiveMessage(message *xml.Message) {
	switch message.Type() {
	case xml.ChatType, xml.NormalType:
		break
	default:
		return
	}
	o.queue.Async(func() {
		o.archiveMessage(message)
	})
}

func (o *ModOffline) DeliverOfflineMessages() {
	o.queue.Async(func() {
		o.deliverOfflineMessages()
	})
}

func (o *ModOffline) archiveMessage(message *xml.Message) {
	toJid := message.ToJID()
	queueSize, err := storage.Instance().CountOfflineMessages(toJid.Node())
	if err != nil {
		log.Error(err)
		return
	}
	exists, err := storage.Instance().UserExists(toJid.Node())
	if err != nil {
		log.Error(err)
		return
	}
	if !exists || queueSize >= o.cfg.QueueSize {
		response := message.Copy()
		response.SetFrom(toJid.String())
		response.SetTo(o.strm.JID().String())
		o.strm.SendElement(response.ServiceUnavailableError())
		return
	}
	delayed := message.Copy()
	delayed.Delay(o.strm.Domain(), "Offline Storage")
	if err := storage.Instance().InsertOfflineMessage(delayed, toJid.Node()); err != nil {
		log.Errorf("%v", err)
		return
	}
	log.Infof("archived offline message... id: %s", message.ID())
}

func (o *ModOffline) deliverOfflineMessages() {
	messages, err := storage.Instance().FetchOfflineMessages(o.strm.Username())
	if err != nil {
		log.Error(err)
		return
	}
	if len(messages) == 0 {
		return
	}
	log.Infof("delivering offline messages... count: %d", len(messages))

	for _, m := range messages {
		o.strm.SendElement(m)
	}
	if err := storage.Instance().DeleteOfflineMessages(o.strm.Username()); err != nil {
		log.Error(err)
	}
}
