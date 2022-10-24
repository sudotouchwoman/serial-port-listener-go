package connection

import (
	"bufio"
	"context"
	"io"
	"log"

	"github.com/tarm/serial"
)

/*
1. Keep track of opened ports
2. Make sure to propagate the error through error chan
3. Run io bound tasks in separate goroutines (reads and writes)
4. Make code independent of io.ReaderWriter source for easier
testing and mocking.
5. Add gorilla/websocket for real-time log tunelling
6. Create React app to display ws data
*/

type SerialConnection struct {
	// Provides some higher-order API to given io.ReadWriter.
	// Basically, populates channels with data and
	// errors collected during scanning
	// Tokenizer attribute can be set to an appropriate function
	// to be used during scanning to tokenize the bytes.
	// bufio.ScanLines is set by default by ConnectionManager
	//  but this property is public thus can be modified by
	// the caller later (please note that modifying makes
	// no sense once the connection starts scanning)
	io.ReadWriter
	context.Context
	Tokenizer bufio.SplitFunc
	DataChan  chan []byte
	errChan   chan error
}

func (ss *SerialConnection) Listen() {
	// Starts scanning provided connection for tokens
	// This method is intended to be run in a separate goroutine
	defer func() {
		close(ss.errChan)
		close(ss.DataChan)
	}()
	scanner := bufio.NewScanner(ss)
	scanner.Split(ss.Tokenizer)
	for scanner.Scan() {
		ss.DataChan <- scanner.Bytes()
	}
	select {
	case <-ss.Done():
		return
	default:
		log.Println("Connection was interrupted before context finished")
		ss.errChan <- scanner.Err()
	}
}

func SerialProvider(baudrate int) ConnectionProvider {
	return func(name string) (wr io.ReadWriter, cancel func(), err error) {
		c := &serial.Config{Name: name, Baud: baudrate}
		stream, err := serial.OpenPort(c)
		return stream, func() { stream.Close() }, err
	}
}
