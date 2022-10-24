package client

import (
	"context"
)

type ConsumerProvider func() []chan<- []byte

func Broadcast(
	producer <-chan []byte,
	provider ConsumerProvider,
	ctx context.Context,
) {
	for {
		select {
		case update, closed := <-producer:
			for _, target := range provider() {
				go func(t chan<- []byte) {
					t <- update
				}(target)
			}
			if closed {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
