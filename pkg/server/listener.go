package server

import (
	"context"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/sudotouchwoman/serial-port-listener-go/pkg/connection"
)

type ListenerServer struct {
	ctx        context.Context
	upgrader   websocket.Upgrader
	serials    *connection.ConnectionManager
	clients    map[*Client]bool
	serialSubs map[string]*Client
}

func (ls *ListenerServer) SocketHandler(w http.ResponseWriter, r *http.Request) {
	// upgrades HTTP connection to a WS one
	ls.upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	conn, err := ls.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error during connection upgrade:", err)
		return
	}
	// create client object
	// and remember to close the connection once done
	defer func() {
		conn.Close()

	}()
	// starts listening for messages via websocket
	// client initiates activity by selecting the serial port to listen to
	// TODO: guess clients should perform some sort of authentication
	// but this can be handled before ws connection is established via middlewares
	// TODO: once nobody is listening to a serial port, it should be closed
	for {
		_, _, readErr := conn.ReadMessage()
		if readErr != nil {
			log.Println("Client Disconnected: ", readErr)
		}
	}
	// do connection-specific things
}

func (ls *ListenerServer) HandleSerialRequest(c *Client, name string) {
	// open the port or respond with an error on failure
	// ...
}
