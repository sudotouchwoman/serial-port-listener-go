package connection

import (
	"bufio"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/sudotouchwoman/serial-port-listener-go/pkg/common"
)

type connHandler struct {
	*SerialConnection
	writer   chan []byte
	connName string
	closed   bool
	close    func() error
}

func (handle *connHandler) ID() string {
	return handle.connName
}

func (handle *connHandler) Data() <-chan []byte {
	return handle.DataChan
}

func (handle *connHandler) Err() <-chan error {
	return handle.errChan
}

var ErrAlreadyClosed = errors.New("this producer has been closed already")

func (handle *connHandler) Close() error {
	if handle.closed {
		return ErrAlreadyClosed
	}
	handle.closed = true
	return handle.close()
}

func (handle *connHandler) Writer() chan<- []byte {
	if handle.writer != nil {
		return handle.writer
	}
	handle.writer = make(chan []byte)
	go func() {
		for d := range handle.writer {
			_, err := handle.SerialConnection.Write(d)
			if err != nil {
				handle.errChan <- err
			}
		}
	}()
	return handle.writer
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
	lock     *sync.RWMutex
	pool     map[string]*connHandler
	provider ConnectionProvider
}

func NewManager(ctx context.Context, p ConnectionProvider) *ConnectionManager {
	return &ConnectionManager{
		lock:     &sync.RWMutex{},
		pool:     map[string]*connHandler{},
		Context:  ctx,
		provider: p,
	}
}

func (cm *ConnectionManager) IsOpen(name string) bool {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	_, open := cm.pool[name]
	return open
}

func (cm *ConnectionManager) Open(name string) (common.DuplexProducer, error) {
	// Return new connection, creating one along the way if it does not exist yet.
	// Propagates errors from provider
	cm.lock.Lock()
	defer cm.lock.Unlock()
	if connection, open := cm.pool[name]; open {
		return connection, nil
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
		handler := &connHandler{
			connName:         name,
			SerialConnection: conn,
			close: func() error {
				ctxCancel()
				canceler()
				return cm.closerFunc(name)
			},
		}
		cm.pool[name] = handler
		// start listening for updates
		go conn.Listen()
		return handler, nil
	}
	return nil, err
}

var ErrConnNotOpened = errors.New("connection does not exist")

func (cm *ConnectionManager) Close(name string) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()
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

func (cm *ConnectionManager) closerFunc(name string) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	connection, open := cm.pool[name]
	if !open {
		return ErrConnNotOpened
	}
	delete(cm.pool, name)
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
