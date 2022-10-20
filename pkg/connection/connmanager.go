package connection

import (
	"bufio"
	"context"
	"errors"
	"io"
	"sync"
)

type connHandler struct {
	*SerialConnection
	Close func()
}

type ConnectionProvider func(string) (wr io.ReadWriter, cancel func(), err error)

type ConnectionManager struct {
	// Manager serves as an API for the connection pool
	// as one may wish to listen to several
	// serial connections simultaneously.
	// Porvider is a dependency injected into manager
	// in order to ease tesing
	// (e.g. to avoid actual interaction with serial connections)
	context.Context
	*sync.Mutex
	pool     map[string]connHandler
	provider ConnectionProvider
}

func NewManager(ctx context.Context, p ConnectionProvider) *ConnectionManager {
	return &ConnectionManager{
		Mutex:    &sync.Mutex{},
		pool:     map[string]connHandler{},
		Context:  ctx,
		provider: p,
	}
}

func (cm *ConnectionManager) Open(name string) (*SerialConnection, error) {
	// Return new connection, creating one along the way if it does not exist yet.
	// Propagates errors from provider
	cm.Lock()
	defer cm.Unlock()
	if connection, open := cm.pool[name]; open {
		return connection.SerialConnection, nil
	}
	wr, canceler, err := cm.provider(name)
	if err == nil && wr != nil {
		ctx, ctxCancel := context.WithCancel(cm)
		conn := &SerialConnection{
			ReadWriter: wr,
			Context:    ctx,
			Tokenizer:  bufio.ScanLines,
			DataChan:   make(chan []byte, 1),
			errChan:    make(chan error, 1),
		}
		cm.pool[name] = connHandler{
			SerialConnection: conn,
			Close: func() {
				canceler()
				ctxCancel()
			},
		}
		return conn, nil
	}
	return nil, err
}

var ErrConnNotOpened = errors.New("connection does not exist")

func (cm *ConnectionManager) Close(name string) error {
	cm.Lock()
	defer cm.Unlock()
	connection, open := cm.pool[name]
	if !open {
		return ErrConnNotOpened
	}
	delete(cm.pool, name)
	connection.Close()
	// check if something went wrong with the connection
	// and propagate the error if any
	// use select to avoid unnesessary blocks
	select {
	case err := <-connection.errChan:
		return err
	default:
		return nil
	}
}
