/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package stream

import (
	"sync"

	"github.com/ortuman/jackal/log"
)

type StreamManager struct {
	sync.RWMutex
	strms       map[string]*Stream
	authedStrms map[string][]*Stream
}

// singleton interface
var (
	instance *StreamManager
	once     sync.Once
)

func Manager() *StreamManager {
	once.Do(func() {
		instance = &StreamManager{
			strms:       make(map[string]*Stream),
			authedStrms: make(map[string][]*Stream),
		}
	})
	return instance
}

func (m *StreamManager) RegisterStream(strm *Stream) {
	log.Infof("registered stream... (id: %s)", strm.ID())
	m.Lock()
	m.strms[strm.ID()] = strm
	m.Unlock()
}

func (m *StreamManager) UnregisterStream(strm *Stream) {
	log.Infof("unregistered stream... (id: %s)", strm.ID())
	m.Lock()
	if authedStrms := m.authedStrms[strm.Username()]; authedStrms != nil {
		authedStrms = removeStreamWithResource(authedStrms, strm.Resource())
		if len(authedStrms) == 0 {
			delete(m.authedStrms, strm.Username())
		}
	}
	m.Unlock()
}

func (m *StreamManager) AuthenticateStream(strm *Stream) {
	log.Infof("authenticated stream... (username: %s)", strm.Username())
	m.Lock()
	if authedStrms := m.authedStrms[strm.Username()]; authedStrms != nil {
		m.authedStrms[strm.Username()] = append(authedStrms, strm)
	} else {
		m.authedStrms[strm.Username()] = []*Stream{strm}
	}
	m.Unlock()
}

func removeStreamWithResource(strms []*Stream, resource string) []*Stream {
	ret := strms[:0]
	for _, s := range strms {
		if s.Resource() != resource {
			ret = append(ret, s)
		}
	}
	return ret
}