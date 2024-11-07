// broker.go
package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"strings"
	"uk.ac.bris.cs/gameoflife/gol/broker/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

// Broker struct to hold the RPC client for the server connection
type Broker struct {
	servers []*rpc.Client
	closing chan struct{}
}

var CloseSystem = false

// NewBroker initializes the broker by connecting to the server
func NewBroker(serverAddrs []string) (*Broker, error) {
	var servers []*rpc.Client
	for _, addr := range serverAddrs {
		client, err := rpc.Dial("tcp", addr)
		if err != nil {
			fmt.Printf("Failed to connect to server at %s: %v\n", addr, err)
			continue
		}
		servers = append(servers, client)
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("failed to connect to any servers")
	}
	return &Broker{servers: servers, closing: make(chan struct{})}, nil
}

// RPC wrapper for CalculateNextState, so the client can call broker.CalculateNextState
func (b *Broker) CalculateNextState(req stubs.Request, res *stubs.Response) error {
	numServers := len(b.servers)
	heightPerServer := (req.EndY - req.StartY) / numServers
	extraHeight := (req.EndY - req.StartY) % numServers
	responses := make([]stubs.Response, numServers)
	errCh := make(chan error, numServers)

	startY := req.StartY
	// Send each part to a different server
	for i := 0; i < numServers; i++ {
		numRows := heightPerServer
		if extraHeight > i {
			numRows++
		}
		endY := startY + numRows

		// Prepare request for each server
		subReq := stubs.Request{
			World:  req.World,
			P:      req.P,
			StartX: req.StartX,
			EndX:   req.EndX,
			StartY: startY,
			EndY:   endY,
			Turn:   req.Turn,
		}

		go func(i int) {
			errCh <- b.servers[i].Call("GOL.CalculateNextState", subReq, &responses[i])
		}(i)
		startY = endY
	}

	// Collect responses
	for i := 0; i < numServers; i++ {
		if err := <-errCh; err != nil {
			return fmt.Errorf("error from server %d: %v", i, err)
		}
	}

	// Aggregate results from responses
	startY = 0
	res.World = func(req stubs.Request) [][]byte {
		newWorld := req.World
		for j := 0; j < numServers; j++ {
			numRows := heightPerServer
			// Add a row if there are extra rows for workers than need doing
			if extraHeight > j {
				numRows++
			}
			endY := startY + numRows
			copy(newWorld[startY:endY], responses[j].World)
			startY = endY
		}
		return newWorld

	}(req)
	res.FlippedCells = func(responses []stubs.Response) []util.Cell {
		var fc []util.Cell
		for _, s := range responses {
			fc = append(fc, s.FlippedCells...)
		}
		return fc
	}(responses)
	return nil
}

func (b *Broker) CalculateAliveCells(req stubs.Request, res *stubs.Response) error {
	numServers := len(b.servers)
	heightPerServer := (req.EndY - req.StartY) / numServers
	extraHeight := (req.EndY - req.StartY) % numServers
	responses := make([]stubs.Response, numServers)
	errCh := make(chan error, numServers)

	startY := req.StartY
	// Send each part to a different server
	for i := 0; i < numServers; i++ {
		numRows := heightPerServer
		if extraHeight > i {
			numRows++
		}
		endY := startY + numRows

		// Prepare request for each server
		subReq := stubs.Request{
			World:  req.World,
			P:      req.P,
			StartX: req.StartX,
			EndX:   req.EndX,
			StartY: startY,
			EndY:   endY,
			Turn:   req.Turn,
		}

		go func(i int) {
			errCh <- b.servers[i].Call("GOL.CalculateAliveCells", subReq, &responses[i])
		}(i)
		startY = endY
	}

	// Collect responses
	for i := 0; i < numServers; i++ {
		if err := <-errCh; err != nil {
			return fmt.Errorf("error from server %d: %v", i, err)
		}
	}
	res.Alive = func(responses []stubs.Response) int {
		alive := 0
		for _, s := range responses {
			alive += s.Alive
		}
		return alive
	}(responses)
	return nil
}

func (b *Broker) ClosingSystem(req stubs.Request, res *stubs.Response) error {
	fmt.Println("Closing all servers and broker...")

	// Close each server connection by calling their CloseSystem method
	for _, server := range b.servers {
		var closeResponse stubs.Response
		err := server.Call("GOL.ClosingSystem", req, &closeResponse)
		if err != nil {
			fmt.Printf("Failed to close server: %v\n", err)
		}
	}

	// Signal broker closure only once
	select {
	case <-b.closing: // Already closed
	default:
		close(b.closing) // Close only once
	}
	return nil
}

// StartRPCServer starts an RPC server for the broker to accept client requests
func StartRPCServer(broker *Broker, brokerAddr string) error {
	server := rpc.NewServer()
	if err := server.Register(broker); err != nil {
		return fmt.Errorf("failed to register broker: %v", err)
	}

	listener, err := net.Listen("tcp", brokerAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", brokerAddr, err)
	}
	fmt.Printf("Broker is listening on %s\n", brokerAddr)

	go func() {
		<-broker.closing
		fmt.Println("Broker is shutting down...")
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-broker.closing:
				return nil // Stop accepting connections when broker is closed
			default:
				fmt.Printf("Error accepting connection: %v\n", err)
				continue
			}
		}
		go server.ServeConn(conn)
	}
}

func main() {
	serversFlag := flag.String("servers", "127.0.0.1:8030", "Comma-separated list of server addresses")
	brokerAddr := flag.String("brokerAddr", "127.0.0.1:8050", "Broker address for client to connect")
	flag.Parse()
	serverAddrs := strings.Split(*serversFlag, ",")
	// Connect broker to the server
	broker, err := NewBroker(serverAddrs)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer func() {
		for _, client := range broker.servers {
			client.Close()
		}
	}()

	// Start the broker's own RPC server for the client
	err = StartRPCServer(broker, *brokerAddr)
	if err != nil {
		fmt.Println("Error starting broker server:", err)
	}
}
