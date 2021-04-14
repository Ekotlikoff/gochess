package gateway

// Credit to https://stackoverflow.com/questions/25484122/map-with-ttl-option-in-go

import (
	"errors"
	"github.com/Ekotlikoff/gochess/internal/server/backend/match"
	"sync"
	"time"
)

type item struct {
	value      *matchserver.Player
	lastAccess int64
}

type TTLMap struct {
	m map[string]*item
	l sync.Mutex
}

func NewTTLMap(ln int, maxTTL int, gcFrequencySecs int) (m *TTLMap) {
	m = &TTLMap{m: make(map[string]*item, ln)}
	go func() {
		gcFrequency := time.Tick(time.Second * time.Duration(gcFrequencySecs))
		for now := range gcFrequency {
			m.l.Lock()
			for k, v := range m.m {
				if now.Unix()-v.lastAccess > int64(maxTTL) {
					delete(m.m, k)
				}
			}
			m.l.Unlock()
		}
	}()
	return
}

func (m *TTLMap) Len() int {
	return len(m.m)
}

func (m *TTLMap) Put(k string, v *matchserver.Player) error {
	m.l.Lock()
	_, ok := m.m[k]
	var it item
	if !ok {
		it := &item{value: v}
		m.m[k] = it
	} else {
		return errors.New("failed to put key: " + k + ", value: " + v.Name())
	}
	it.lastAccess = time.Now().Unix()
	m.l.Unlock()
	return nil
}

func (m *TTLMap) Get(k string) (v *matchserver.Player, err error) {
	m.l.Lock()
	if it, ok := m.m[k]; ok {
		v = it.value
		it.lastAccess = time.Now().Unix()
	} else {
		err = errors.New("failed to get")
	}
	m.l.Unlock()
	return

}

func (m *TTLMap) Refresh(k, newk string) error {
	m.l.Lock()
	it, ok := m.m[k]
	if ok {
		it.lastAccess = time.Now().Unix()
	}
	if _, newok := m.m[newk]; !newok && ok {
		m.m[newk] = it
		delete(m.m, k)
	} else {
		return errors.New("failed to refresh key")
	}
	m.l.Unlock()
	return nil
}
