package client

import (
	"context"

	"github.com/sudotouchwoman/serial-port-listener-go/pkg/common"
)

type ConsumerProvider func() []common.RecieverChan

type BroadcastedMessage struct {
	Text       string `json:"text"`
	ProducerID string `json:"producer_id"`
}

func Broadcast(
	ctx context.Context,
	producerID string,
	producer <-chan []byte,
	provider ConsumerProvider,
) {
	// wait for updates or until the channels gets closed
	// this function is intended to be run in a separate gorourine
	for {
		select {
		case update, open := <-producer:
			emission := BroadcastedMessage{
				Text:       string(update),
				ProducerID: producerID,
			}
			for _, target := range provider() {
				go func(t common.RecieverChan) {
					// emit a marshallable message
					t <- emission
				}(target)
			}
			if !open {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func GeneralBroadcast(
	ctx context.Context,
	producer common.UpdatesChan,
	provider ConsumerProvider) {
	for {
		select {
		case update, open := <-producer:
			for _, target := range provider() {
				go func(t common.RecieverChan) {
					// emit a marshallable message
					t <- update
				}(target)
			}
			if !open {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
