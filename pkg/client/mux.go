package client

import (
	"context"

	"github.com/sudotouchwoman/serial-port-listener-go/pkg/common"
)

type ConsumerProvider func() []common.RecieverChan

func Broadcast(
	ctx context.Context,
	producer <-chan []byte,
	provider ConsumerProvider,
) {
	// wait for updates or until the channels gets closed
	// this function is intended to be run in a separate gorourine
	for {
		select {
		case update, open := <-producer:
			for _, target := range provider() {
				go func(t chan<- []byte) {
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
