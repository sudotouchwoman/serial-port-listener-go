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

type ClientConsumer struct {
	// this struct implements Consumer
	// interface and can thus be used with
	// ConsumerManager. ListenerServer stores
	// a map of all clients just in case
	Socket     *websocket.Conn
	Token      string
	socketChan chan []byte
}

func (c *ClientConsumer) ID() string {
	return c.Token
}

func (c *ClientConsumer) Reciever() common.RecieverChan {
	if c.socketChan != nil {
		return c.socketChan
	}
	c.socketChan = make(chan []byte)
	return c.socketChan
}

type ListenerServer struct {
	mu       *sync.RWMutex
	ctx      context.Context
	upgrader websocket.Upgrader
	conns    common.ProducerManager
	subs     common.ConsumerManager
	clients  map[*ClientConsumer]bool
}

func (ls *ListenerServer) SocketHandler(w http.ResponseWriter, r *http.Request) {
	// upgrades HTTP connection to a WS one
	ls.upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	conn, err := ls.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error during connection upgrade:", err)
		return
	}
	// create new consumer struct to store websocket
	// and u4 token
	c := &ClientConsumer{
		Socket:     conn,
		Token:      uuid.NewString(),
		socketChan: make(chan []byte),
	}
	ls.mu.Lock()
	ls.clients[c] = true
	ls.mu.Unlock()
	// listen for updates and
	// redirect them into the websocket
	go func() {
		for update := range c.socketChan {
			msg, wrapErr := ls.messageWrapper(c, update)
			if wrapErr != nil {
				go log.Println("Error on message wrap:", wrapErr, " Client:", c.Token)
				continue
			}
			if err := c.Socket.WriteMessage(0, msg); err != nil {
				// is it okay to log in a goroutine?
				go log.Println("Error on send to ws:", err, " Client:", c.Token)
			}
		}
	}()
	// remember to close channels and connections once done
	defer func() {
		close(c.socketChan)
		if err := conn.Close(); err != nil {
			log.Println("Error during closing websocket", err)
		}
		ls.subs.DropConsumer(c)
		delete(ls.clients, c)
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

func (ls *ListenerServer) HandleSocketMessage(c *ClientConsumer, payload []byte) {
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
				log.Println(err)
				return
			}
			ls.subs.Subscribe(c, conn)
		}
		return
	}
}

func (ls *ListenerServer) messageWrapper(c *ClientConsumer, data []byte) (msg []byte, err error) {
	response := SockResponse{
		Body: string(data),
	}
	msg, err = json.Marshal(response)
	return
}
