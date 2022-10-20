package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/tarm/serial"
)

func main() {
	com, baudrate, timeout := "COM6", 115200, 30
	flag.StringVar(&com, "com", com, "Serial Port Name.")
	flag.IntVar(&timeout, "timeout", timeout, "Listening Timeout (seconds)")
	flag.IntVar(&baudrate, "b", baudrate, "Serial Port Baudrate.")
	flag.Parse()

	if timeout <= 0 {
		fmt.Println("Timeout should be positive")
		return
	}
	if baudrate <= 0 {
		fmt.Println("Baudrate should be positive")
		return
	}

	log.Printf(
		"Starts probing %s on baudrate %d for %d seconds\n",
		com, baudrate, timeout)

	c := &serial.Config{Name: com, Baud: baudrate}
	stream, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}

	duration := time.Duration(timeout) * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(duration, func() {
		log.Println("Closing serial port stream")
		cancel()
		stream.Close()
	})
	defer cancel()
	defer stream.Close()

	dataCh := make(chan []byte, 1)
	errChan := make(chan error, 1)

	go func(ctx context.Context) {
		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			// do something with buffer contents
			// here the data is just written to stdout
			dataCh <- scanner.Bytes()
		}
		defer close(dataCh)
		defer close(errChan)
		select {
		case <-ctx.Done():
			log.Println("Scanner stopped")
			return
		default:
			log.Println("Stream was closed or an error occured")
			errChan <- scanner.Err()
		}
	}(ctx)

	for data := range dataCh {
		log.Println(string(data))
	}
	log.Println("Finished probing")

	// generally parsing can be done
	// with bufio using Split attribute
	if err, open := <-errChan; open && err != nil {
		log.Fatal(err)
	}
}
