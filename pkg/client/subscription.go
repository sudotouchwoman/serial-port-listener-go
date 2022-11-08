package client

import (
	"context"
	"log"
	"sync"

	"github.com/sudotouchwoman/serial-port-listener-go/pkg/common"
)

type links map[common.Consumer]bool

// Manages state of subscriptions
// Asks external service to close producer
// in case it is not requested by
// clients anymore.
type ConsumerManager struct {
	ctx       context.Context
	mu        sync.RWMutex
	listeners map[common.Producer]links
}

func NewManager() *ConsumerManager {
	return &ConsumerManager{
		listeners: map[common.Producer]links{},
	}
}

func (sm *ConsumerManager) IsSub(c common.Consumer, p common.Producer) bool {
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

func (sm *ConsumerManager) GetListeners(p common.Producer) common.Consumers {
	// Checks all clients subscribed for given producer
	// is intended to for usage with broadcaster
	// to check targets to send data to
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if _, ok := sm.listeners[p]; !ok {
		return common.Consumers{}
	}
	links := sm.listeners[p]
	listenters := make(common.Consumers, 0, len(links))
	for l := range links {
		listenters = append(listenters, l)
	}
	return listenters
}

func (sm *ConsumerManager) recievers(p common.Producer) []common.RecieverChan {
	// Checks all clients subscribed for given producer
	// is intended to for usage with broadcaster
	// to check targets to send data to
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if _, ok := sm.listeners[p]; !ok {
		return []common.RecieverChan{}
	}
	listenters := sm.GetListeners(p)
	if len(listenters) == 0 {
		return []common.RecieverChan{}
	}
	consumers := make([]common.RecieverChan, 0, len(listenters))
	for _, l := range listenters {
		consumers = append(consumers, l.Reciever())
	}
	return consumers
}

func (sm *ConsumerManager) Subscribe(c common.Consumer, p common.Producer) {
	// Subscribes common.Consumer c to updates from
	// common.Producer p. In case given producer does not
	// exist yet, start broadcasting its updates
	if c == nil || p == nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.listeners[p]; !ok {
		sm.listeners[p] = links{c: true}
		// if this is the first consumer for given producer,
		// start broadcasting updates
		go Broadcast(
			sm.ctx,
			p.ID(),
			p.Data(),
			func() []common.RecieverChan {
				return sm.recievers(p)
			},
		)
		return
	}
	producer := sm.listeners[p]
	producer[c] = true
}

func (sm *ConsumerManager) Unsubscribe(c common.Consumer, p common.Producer) {
	// code duplication in nil checks sucks
	if c == nil || p == nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if producerLinks, ok := sm.listeners[p]; ok {
		delete(producerLinks, c)
		// close the producer if there is nobody left
		// listening
		// this should also stop the broadcasting goroutine
		// created in Subscribe
		if len(producerLinks) == 0 {
			go func() {
				if err := p.Close(); err != nil {
					log.Println("Error on Producer.Close():", err)
				}
			}()
		}
	}
}

func (sm *ConsumerManager) DropConsumer(c common.Consumer) {
	// unsubscribes given common.Consumer c from updates
	// from each of producers
	if c == nil {
		return
	}
	// this might be slow, I guess
	for producer := range sm.listeners {
		sm.Unsubscribe(c, producer)
	}
}
