package server

import "encoding/json"

type SockRequest struct {
	Token   string          `json:"token"`
	MType   string          `json:"mtype"`
	Payload json.RawMessage `json:"payload"`
}

type MessagePort struct {
	Serial string `json:"serial"`
	Action string `json:"action"`
}

const (
	MsgPort   = "port"
	ClosePort = "close"
	OpenPort  = "open"
)

type SockResponse struct {
	Body  string `json:"body"`
	Error string `json:"error,omitempty"`
}
