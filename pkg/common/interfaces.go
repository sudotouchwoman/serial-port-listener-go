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

type RecieverChan chan<- []byte

type Consumer interface {
	ID() string
	Reciever() RecieverChan
}

type Producer DuplexProducer
type Consumers []Consumer

type ConsumerManager interface {
	IsSub(Consumer, Producer) bool
	GetListeners(Producer) Consumers
	GetRecievers() []RecieverChan
	Subscribe(Consumer, Producer)
	Unsubscribe(Consumer, Producer)
	DropConsumer(Consumer)
}
