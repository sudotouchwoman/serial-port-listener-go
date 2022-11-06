package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"

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
	updatesChan chan []byte
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

func NewClientConsumer() Client {
	return &ClientConsumer{
		token:       uuid.NewString(),
		updatesChan: make(chan []byte),
	}
}

type ListenerServer struct {
	mu            *sync.RWMutex
	ctx           context.Context
	upgrader      websocket.Upgrader
	conns         common.ProducerManager
	subs          common.ConsumerManager
	clients       map[Client]bool
	clientFactory func() Client
}

func (ls *ListenerServer) SocketHandler(w http.ResponseWriter, r *http.Request) {
	// upgrades HTTP connection to a WS one
	ls.upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	conn, err := ls.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error during connection upgrade:", err)
		http.Error(w, "Failed to upgrade connection", 422)
		return
	}
	// create new consumer struct to store websocket
	// and u4 token
	c := ls.clientFactory()
	ls.mu.Lock()
	ls.clients[c] = true
	ls.mu.Unlock()
	// listen for updates and
	// redirect them into the websocket
	go func() {
		for update := range c.Updates() {
			// wrap messages into a json response
			msg, errMsgWrap := ls.messageWrapper(c, update)
			if errMsgWrap != nil {
				go log.Println("Error on message wrap:", errMsgWrap, " Client:", c.ID())
				continue
			}
			if errWrite := conn.WriteMessage(0, msg); errWrite != nil {
				// is it okay to log in a goroutine?
				go log.Println("Error on send to ws:", errWrite, " Client:", c.ID())
			}
		}
	}()
	// remember to close channels and connections once done
	defer func() {
		close(c.Reciever())
		if err := conn.Close(); err != nil {
			log.Println("Error during closing websocket", err)
		}
		ls.subs.DropConsumer(c)
		ls.mu.Lock()
		delete(ls.clients, c)
		ls.mu.Unlock()
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
}

func (ls *ListenerServer) messageWrapper(c Client, data []byte) (msg []byte, err error) {
	response := SockResponse{
		Body: string(data),
	}
	msg, err = json.Marshal(response)
	return
}
