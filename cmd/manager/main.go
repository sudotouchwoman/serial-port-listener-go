package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/sudotouchwoman/serial-port-listener-go/pkg/connection"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := connection.NewManager(ctx, connection.SerialProvider(baudrate))
	duration := time.Duration(timeout) * time.Second
	time.AfterFunc(duration, func() {
		log.Println("Closing serial port stream")
		// somehow this cancel does not modify the behaviour
		// cancel()
		if err := manager.Close(com); err != nil {
			log.Println("Error during scanning", err)
		}
	})

	conn, err := manager.Open(com)
	if err != nil {
		log.Fatal(err)
	}

	for data := range conn.Data() {
		log.Println(string(data))
	}
	log.Println("Finished probing")
}
