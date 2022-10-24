package client

import (
	"sync"
)

type Links map[*Client]bool
type Clients []*Client
type Producer string
type Closer func(Producer)

type SubscriptionManager struct {
	// Manages state of subscriptions
	// Asks external service to close producer
	// in case it is not requested by
	// clients anymore
	mu        *sync.RWMutex
	listeners map[Producer]Links
	Closer
}

func NewManager(c Closer) *SubscriptionManager {
	return &SubscriptionManager{
		mu:        &sync.RWMutex{},
		listeners: map[Producer]Links{},
		Closer:    c,
	}
}

func (sm *SubscriptionManager) IsSub(c *Client, p Producer) bool {
	// Checks whether given client is subscribed for given producer
	if c == nil {
		return false
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if _, ok := sm.listeners[p]; !ok {
		return false
	}
	producer := sm.listeners[p]
	_, ok := producer[c]
	return ok
}

func (sm *SubscriptionManager) GetListeners(p Producer) []*Client {
	// Checks all clients subscribed for given producer
	// is intended to for usage with broadcaster
	// to check targets to send data to
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if _, ok := sm.listeners[p]; !ok {
		return []*Client{}
	}
	links := sm.listeners[p]
	listenters := make([]*Client, 0, len(links))
	for l := range links {
		listenters = append(listenters, l)
	}
	return listenters
}

func (sm *SubscriptionManager) Subscribe(c *Client, p Producer) {
	if c == nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.listeners[p]; !ok {
		sm.listeners[p] = Links{c: true}
		return
	}
	producer := sm.listeners[p]
	producer[c] = true
}

func (sm *SubscriptionManager) Unsubscribe(c *Client, p Producer) {
	// code duplication in nil checks sucks
	if c == nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if producer, ok := sm.listeners[p]; ok {
		delete(producer, c)
		// close the producer if there is nobody left
		// listening
		if len(producer) == 0 {
			go sm.Closer(p)
		}
	}
}

func (sm *SubscriptionManager) DropClient(c *Client) {
	if c == nil {
		return
	}
	// this might be slow, I guess
	for producer := range sm.listeners {
		sm.Unsubscribe(c, producer)
	}
}
