package client

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	// serials this client is listening to
	Token     string
	Socket    *websocket.Conn
	WriteLock *sync.Mutex
}

func New(token string, conn *websocket.Conn) *Client {
	return &Client{
		Token:     token,
		Socket:    conn,
		WriteLock: &sync.Mutex{},
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

func (c *Client) Send(mtype int, msg []byte) error {
	c.WriteLock.Lock()
	defer c.WriteLock.Unlock()
	return c.Socket.WriteMessage(mtype, msg)
}

func (c *Client) SendJSON(msg interface{}) error {
	c.WriteLock.Lock()
	defer c.WriteLock.Unlock()
	return c.Socket.WriteJSON(msg)
}
