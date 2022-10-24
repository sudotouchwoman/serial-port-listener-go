package client

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	Socket *websocket.Conn
	mu     *sync.Mutex
}

func (c *Client) Send(mtype int, msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Socket.WriteMessage(mtype, msg)
}

func (c *Client) SendJSON(msg interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Socket.WriteJSON(msg)
}

func New(conn *websocket.Conn) *Client {
	return &Client{
		Socket: conn,
		mu:     &sync.Mutex{},
	}
}

const (
	MsgPort   = "port"
	ClosePort = "close"
	OpenPort  = "open"
)

type Message struct {
	Token   string          `json:"token"`
	MType   string          `json:"mtype"`
	Payload json.RawMessage `json:"payload"`
}

type MessagePort struct {
	Serial string `json:"serial"`
	Action string `json:"action"`
}
