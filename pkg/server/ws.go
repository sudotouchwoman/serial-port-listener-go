package server

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	// serials this client is listening to
	Token         string
	Subscriptions map[string]bool
	Socket        *websocket.Conn
	WriteLock     *sync.Mutex
}

func (c *Client) Send(mtype int, msg []byte) error {
	c.WriteLock.Lock()
	defer c.WriteLock.Unlock()
	return c.Socket.WriteMessage(mtype, msg)
}
