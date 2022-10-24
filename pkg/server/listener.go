package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/sudotouchwoman/serial-port-listener-go/pkg/client"
	"github.com/sudotouchwoman/serial-port-listener-go/pkg/connection"
)

type ListenerServer struct {
	ctx      context.Context
	upgrader websocket.Upgrader
	conns    *connection.ConnectionManager
	subs     *client.SubscriptionManager
	clients  map[*client.Client]bool
}

func (ls *ListenerServer) SocketHandler(w http.ResponseWriter, r *http.Request) {
	// upgrades HTTP connection to a WS one
	ls.upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	conn, err := ls.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error during connection upgrade:", err)
		return
	}
	var c *client.Client = nil
	// create client object
	// and remember to close the connection once done
	defer func() {
		conn.Close()
		ls.subs.DropClient(c)
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

func (ls *ListenerServer) HandleSocketMessage(c *client.Client, payload []byte) {
	msg := client.Message{}
	if err := json.Unmarshal(payload, &msg); err != nil {
		log.Println("Failed to decode ws message:", err)
		return
	}
	if msg.MType == client.MsgPort {
		msgPort := client.MessagePort{}
		if err := json.Unmarshal(msg.Payload, &msgPort); err != nil {
			log.Println("Failed to decode port message:", err)
			return
		}
		portOpened := ls.conns.IsOpen(msgPort.Serial)
		if msgPort.Action == client.OpenPort {
			conn, err := ls.conns.Open(msgPort.Serial)
			if err != nil {
				// handle error, send formatted message to user
				log.Println(err)
				return
			}
			ls.subs.Subscribe(c, client.Producer(msgPort.Serial))
			// start listening to connection
			// data will be stored in conn.DataChan
			// also start reading from the connection
			if !portOpened {
				// go client.Broadcast(conn.DataChan, func() []chan<- []byte {
				// 	// checks for clients listening to this port
				// 	listeners := ls.subs.GetListeners(client.Producer(msgPort.Serial))
				// 	if len(listeners) == 0 {
				// 		return []chan<- []byte{}
				// 	}
				// 	for c := range
				// }, ls.ctx)
				go conn.Listen()
			}
		}
	}
}

func (ls *ListenerServer) HandleSerialRequest(c *client.Client, p client.Producer) {
	// open the port or respond with an error on failure
	// ...
	// if _, ok := c.Subscriptions[name]; ok {
	// 	// client already subscribed, nothing to do
	// 	return
	// }
	// this serial port is already opened (and listened to)
	// client should get subscribed to updates then
	// if ls.conns.IsOpen(name) {
	// 	return
	// }
	// if port, err := ls.serials.Open(name); err == nil {
	// 	c.Subscriptions[name] = true
	// 	// like this? user
	// 	go port.Listen()
	// }
}
