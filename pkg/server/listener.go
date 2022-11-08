package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sudotouchwoman/serial-port-listener-go/pkg/client"
	"github.com/sudotouchwoman/serial-port-listener-go/pkg/common"
)

type Client interface {
	common.Consumer
	Updates() common.UpdatesChan
	Stop(common.UpdatesChan)
}

type ClientConsumer struct {
	// this struct implements Consumer
	// interface and can thus be used with
	// ConsumerManager. ListenerServer stores
	// a map of all clients just in case
	token       string
	updatesChan chan interface{}
	sessions    map[common.UpdatesChan]chan interface{}
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

func (c *ClientConsumer) ID() string {
	return c.token
}

func (c *ClientConsumer) Stop(u common.UpdatesChan) {
	c.mu.Lock()
	if ch, exists := c.sessions[u]; exists {
		close(ch)
	}
	delete(c.sessions, u)
	c.mu.Unlock()
	// essentially stops the infinite reading loop
	// created in the constructor
	// and closes all reader channels
	c.cancel()
	go func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		for _, s := range c.sessions {
			close(s)
		}
	}()
}

func (c *ClientConsumer) Reciever() common.RecieverChan {
	return c.updatesChan
}

// only intended to be called once per handler
// as this creates a new subchannel for updates broadcast
func (c *ClientConsumer) Updates() common.UpdatesChan {
	c.mu.Lock()
	defer c.mu.Unlock()
	newUpdateChan := make(chan interface{})
	c.sessions[newUpdateChan] = newUpdateChan
	return newUpdateChan
}

func NewClientConsumer(id string) Client {
	ctx, cancel := context.WithCancel(context.Background())
	incomingUpdates := make(chan interface{})
	c := &ClientConsumer{
		token:       id,
		updatesChan: incomingUpdates,
		ctx:         ctx,
		cancel:      cancel,
		sessions:    map[common.UpdatesChan]chan interface{}{},
	}
	go client.GeneralBroadcast(ctx, incomingUpdates, func() []common.RecieverChan {
		c.mu.RLock()
		defer c.mu.RUnlock()
		if len(c.sessions) == 0 {
			return []common.RecieverChan{}
		}
		// it's a shame compiler does not let me pass this one
		// like, slice of chan interface{} pointers is inconvertible to
		// a slice of chan<- interface{} pointers, damn
		recv := make([]common.RecieverChan, 0, len(c.sessions))
		for _, u := range c.sessions {
			recv = append(recv, u)
		}
		return recv
	})
	return c
}

// Performs 2 main tasks:
// provides a handler for websocket endpoint and
// dispatches events incoming from the socket
// internally communicates with ProducerManager and ConsumerManager,
// which in turn are responsible for connection/client creation
type ListenerServer struct {
	mu             sync.RWMutex
	ctx            context.Context
	upgrader       websocket.Upgrader
	conns          common.ProducerManager
	subs           common.SubscriptionManager
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

	updates := c.Updates()
	defer c.Stop(updates)
	// listen for updates and
	// redirect them into the websocket
	go func() {
		for update := range updates {
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
		// this was last of this client's sessions
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
