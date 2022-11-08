package common

import (
	"context"
)

type DuplexProducer interface {
	ID() string
	Data() <-chan []byte
	Err() <-chan error
	Close() error
	Writer() chan<- []byte
}

type ProducerFactory func(ctx context.Context) DuplexProducer

type ProducerManager interface {
	IsOpen(string) bool
	Open(string) (DuplexProducer, error)
	Close(string) error
}

type RecieverChan chan<- interface{}
type UpdatesChan <-chan interface{}

type Consumer interface {
	ID() string
	Reciever() RecieverChan
}

type Producer DuplexProducer
type Consumers []Consumer

type SubscriptionManager interface {
	IsSub(Consumer, Producer) bool
	GetListeners(Producer) Consumers
	Subscribe(Consumer, Producer)
	Unsubscribe(Consumer, Producer)
	DropConsumer(Consumer)
}

type CtxKey string

const ClientIDKey CtxKey = "clientID"
