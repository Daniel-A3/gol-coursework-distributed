// broker.go
package main

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"uk.ac.bris.cs/gameoflife/gol/server/stubs"
)

// Broker struct to hold the RPC client for the server connection
type Broker struct {
	client *rpc.Client
}

// NewBroker initializes the broker by connecting to the server
func NewBroker(serverAddr string) (*Broker, error) {
	client, err := rpc.Dial("tcp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %v", err)
	}
	return &Broker{client: client}, nil
}

// RPC wrapper for CalculateNextState, so the client can call broker.CalculateNextState
func (b *Broker) CalculateNextState(req stubs.Request, res *stubs.Response) error {
	return b.client.Call("GOL.CalculateNextState", req, res)
}

func (b *Broker) CalculateAliveCells(req stubs.Request, res *stubs.Response) error {
	return b.client.Call("GOL.CalculateAliveCells", req, res)
}

// StartRPCServer starts an RPC server for the broker to accept client requests
func StartRPCServer(broker *Broker, brokerAddr string) error {
	server := rpc.NewServer()
	err := server.Register(broker)
	if err != nil {
		return fmt.Errorf("failed to register broker: %v", err)
	}

	listener, err := net.Listen("tcp", brokerAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", brokerAddr, err)
	}
	defer listener.Close()

	fmt.Printf("Broker is listening on %s\n", brokerAddr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection: %v\n", err)
			continue
		}
		server.ServeConn(conn)
	}
}

func main() {
	serverAddr := "127.0.0.1:8030" // Server address
	brokerAddr := "127.0.0.1:8050" // Broker address for client to connect

	// Connect broker to the server
	broker, err := NewBroker(serverAddr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer broker.client.Close()

	// Start the broker's own RPC server for the client
	err = StartRPCServer(broker, brokerAddr)
	if err != nil {
		fmt.Println("Error starting broker server:", err)
	}
}
