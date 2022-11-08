package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sudotouchwoman/serial-port-listener-go/pkg/common"
)

type Client interface {
	common.Consumer
	Updates() common.UpdatesChan
}

type ClientConsumer struct {
	// this struct implements Consumer
	// interface and can thus be used with
	// ConsumerManager. ListenerServer stores
	// a map of all clients just in case
	token       string
	updatesChan chan interface{}
}

func (c *ClientConsumer) ID() string {
	return c.token
}

func (c *ClientConsumer) Reciever() common.RecieverChan {
	return c.updatesChan
}

func (c *ClientConsumer) Updates() common.UpdatesChan {
	return c.updatesChan
}

func NewClientConsumer(id string) Client {
	return &ClientConsumer{
		token:       id,
		updatesChan: make(chan interface{}),
	}
}

type ListenerServer struct {
	mu             sync.RWMutex
	ctx            context.Context
	upgrader       websocket.Upgrader
	conns          common.ProducerManager
	subs           common.ConsumerManager
	clientSessions map[Client]int
	clients        map[string]Client
	clientFactory  func(id string) Client
}

func (ls *ListenerServer) SocketHandler(w http.ResponseWriter, r *http.Request) {
	// check if this user is currently connected
	// from somewhere else
	clientID, foundInCtx := r.Context().Value(common.ClientIDKey).(string)
	if !foundInCtx {
		// essentially, auth error
		// client id should be passed externally
		http.Error(w, "Must be logged in to proceed", http.StatusForbidden)
		return
	}
	// try to pick up client from the storage
	var c Client = nil
	ls.mu.RLock()
	if _, clientFound := ls.clients[clientID]; !clientFound {
		ls.mu.RUnlock()
		// new user might have connected:
		// create a corresponding entry
		c = ls.clientFactory(clientID)
		ls.mu.Lock()
		ls.clientSessions[c]++
		ls.clients[clientID] = c
		ls.mu.Unlock()
	} else {
		// this is a bit messy due to Go's
		// variable scopes. By this moment goroutine still
		// holds the Read lock thus the following line is legal
		// we have to explicitly RUnlock() in the main branch though
		// as it should be followed by a Write-locking operation
		c = ls.clients[clientID]
		ls.mu.RUnlock()
	}
	// upgrades HTTP connection to a WS one
	ls.upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	conn, err := ls.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error during connection upgrade:", err)
		http.Error(w, "Failed to upgrade connection", http.StatusUnprocessableEntity)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Println("Error during closing websocket", err)
		}
	}()
	// listen for updates and
	// redirect them into the websocket
	go func() {
		for update := range c.Updates() {
			// wrap messages into a json response
			// and fire
			if errWrite := conn.WriteJSON(update); errWrite != nil {
				// is it okay to log in a goroutine?
				go log.Println("Error on send to ws:", errWrite, " Client:", clientID)
			}
		}
	}()
	// remember to close channels and connections once done
	defer func() {
		close(c.Reciever())
		// only drop this client
		// if the last connection is being closed
		// (one might like to still listen to updates
		// in another tab or whatever)
		ls.mu.Lock()
		ls.clientSessions[c]--
		if ls.clientSessions[c] == 0 {
			delete(ls.clientSessions, c)
			ls.mu.Unlock()
			ls.subs.DropConsumer(c)
		}
	}()
	// starts listening for messages via websocket
	// client initiates activity by selecting the serial port to listen to
	// TODO: guess clients should perform some sort of authentication
	// but this can be handled before ws connection is established via middlewares
	// TODO: once nobody is listening to a serial port, it should be closed
	for {
		select {
		case <-ls.ctx.Done():
			return
		default:
			_, message, readErr := conn.ReadMessage()
			if readErr != nil {
				log.Println("Client Disconnected: ", readErr)
				return
			}
			ls.HandleSocketMessage(c, message)
		}
	}
}

func (ls *ListenerServer) HandleSocketMessage(c Client, payload []byte) {
	msg := SockRequest{}
	if err := json.Unmarshal(payload, &msg); err != nil {
		log.Println("Failed to decode ws message:", err)
		return
	}
	if msg.MType == MsgPort {
		msgPort := MessagePort{}
		if err := json.Unmarshal(msg.Payload, &msgPort); err != nil {
			log.Println("Failed to decode port message:", err)
			return
		}
		if msgPort.Action == OpenPort {
			conn, err := ls.conns.Open(msgPort.Serial)
			if err != nil {
				// handle error, send formatted message to user
				// refactoring idea: wrap the error here,
				// avoid parsing on upper levels
				c.Reciever() <- []byte(err.Error())
				log.Println(err)
				return
			}
			// subscribe and inform the user
			ls.subs.Subscribe(c, conn)
			c.Reciever() <- []byte("subscribed to producer:" + msgPort.Serial)
			return
		}
		if msgPort.Action == ClosePort {
			if !ls.conns.IsOpen(msgPort.Serial) {
				// this one wasn't opened in the first place
				c.Reciever() <- []byte("this producer is inactive:" + msgPort.Serial)
				return
			}
			// still have to call this method, as
			// the producer is not stored anywhere except
			// the manager
			conn, err := ls.conns.Open(msgPort.Serial)
			if err != nil {
				c.Reciever() <- []byte(err.Error())
				log.Println(err)
				return
			}
			// unsubscribe
			ls.subs.Unsubscribe(c, conn)
			c.Reciever() <- []byte("unsubscribed from producer:" + msgPort.Serial)
			return
		}
		c.Reciever() <- []byte("unknown action")
		log.Println("Unknown action for port request:", msgPort)
		return
	}
	// handle other types of messages
	log.Println("Unknown request type:", msg)
}
