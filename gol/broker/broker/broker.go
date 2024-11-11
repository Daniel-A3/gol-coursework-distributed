// broker.go
package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/gol/broker/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

// Broker struct to hold the RPC client for the server connection
type Broker struct {
	servers      []*rpc.Client
	callbackAddr string
	closedLM     bool
	closing      chan struct{}
	paused       bool
	pausedCond   *sync.Cond
	serverMutex  sync.Mutex
}

var mu sync.Mutex
var worldTurn [][]byte
var fTurn int
var muNotify sync.Mutex

// NewBroker initializes the broker by connecting to the server
func NewBroker() (*Broker, error) {
	var servers []*rpc.Client
	for _, addr := range serverAddrs {
		client, err := rpc.Dial("tcp", addr)
		if err != nil {
			fmt.Printf("Failed to connect to server at %s: %v\n", addr, err)
			continue
		}
		servers = append(servers, client)
	}
	return &Broker{servers: servers, closing: make(chan struct{}), pausedCond: sync.NewCond(&mu)}, nil
}

// AddServer registers a new server with the broker
func (b *Broker) AddServer(req stubs.BrokerRequest, res *stubs.BrokerResponse) error {
	b.serverMutex.Lock()
	defer b.serverMutex.Unlock()
	serverAddrs = append(serverAddrs, req.Addr)
	client, err := rpc.Dial("tcp", req.Addr)
	if err != nil {
		return fmt.Errorf("Failed to connect to new server at %s: %v", req.Addr, err)
	}
	b.servers = append(b.servers, client)
	fmt.Printf("New server added at %s\n", req.Addr)
	return nil
}

// RemoveDisconnectedServer removes a server that has been disconnected
func (b *Broker) RemoveDisconnectedServer(index int) {
	b.serverMutex.Lock()
	defer b.serverMutex.Unlock()

	if index >= 0 && index < len(b.servers) {
		b.servers = append(b.servers[:index], b.servers[index+1:]...)
		fmt.Println("before removing", serverAddrs)
		serverAddrs = append(serverAddrs[:index], serverAddrs[index+1:]...)
		fmt.Printf("Server at index %d removed due to disconnection\n", index)
	} else {
		fmt.Printf("Attempted to remove a server at an invalid index: %d\n", index)
	}
}

func (b *Broker) healthCheck() {
	for i := len(b.servers) - 1; i >= 0; i-- { // Iterate in reverse order
		if !b.isServerAlive(b.servers[i]) {
			// Remove the server if it's unresponsive
			b.RemoveDisconnectedServer(i)
			fmt.Println("New server list: ", serverAddrs)
		}
	}
}

func (b *Broker) isServerAlive(client *rpc.Client) bool {
	err := client.Call("GOL.Ping", stubs.Request{}, &stubs.Response{})
	return err == nil
}

func (b *Broker) RegisterCallback(req stubs.RequestEvent, res *struct{}) error {
	b.callbackAddr = req.CallbackAddr
	return nil
}

func (b *Broker) NotifyTurnComplete(turn int, flipped []util.Cell) error {
	muNotify.Lock()
	if b.callbackAddr == "" {
		return fmt.Errorf("callback address not set")
	}
	client, err := rpc.Dial("tcp", b.callbackAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to callback address: %v", err)
	}
	defer client.Close()
	req := stubs.RequestEvent{TurnDone: turn, FlippedCells: flipped}
	res := stubs.ResponseEvent{}
	client.Call("EventReceiver.TurnCompleteEvent", req, res)
	muNotify.Unlock()
	return nil
}

func (b *Broker) SendWorld(req stubs.Request, res *stubs.Response) error {
	mu.Lock()
	res.World = worldTurn
	res.Turn = fTurn
	mu.Unlock()
	return nil
}

func (b *Broker) Ticker(req stubs.Request, res *stubs.Response) error {
	mu.Lock()
	res.Turn = fTurn
	req.World = worldTurn
	if fTurn != 0 {
		b.CalculateAliveCells(req, res)
	}
	mu.Unlock()
	return nil
}

func (b *Broker) CalculateTurns(req stubs.Request, res *stubs.Response) error {
	numTurns := req.Turns // Number of turns to process
	world := req.World
	fTurn = 0
	//var muTurn sync.Mutex

	for turn := 0; turn < numTurns; turn++ {
		mu.Lock()
		for b.paused {
			b.pausedCond.Wait()
		}
		mu.Unlock()
		// Prepare a request for a single turn
		reqTurn := stubs.Request{
			World:     world,
			P:         req.P,
			StartX:    req.StartX,
			EndX:      req.EndX,
			StartY:    req.StartY,
			EndY:      req.EndY,
			TurnDoing: turn + 1,
		}

		// Call CalculateNextState for each turn
		resTurn := new(stubs.Response)
		err := b.CalculateNextState(reqTurn, resTurn)
		if err != nil {
			fmt.Printf("error processing turn %d: %v\n", turn+1, err)
			break
		}
		if len(b.servers) == 0 {
			break
		}

		// Update world with the response for the next turn
		mu.Lock()
		world, worldTurn = resTurn.World, resTurn.World
		res.FlippedCells = resTurn.FlippedCells
		fTurn = turn + 1
		b.NotifyTurnComplete(fTurn, res.FlippedCells)
		mu.Unlock()
		if b.closedLM {
			break
		}

	}
	b.closedLM = false
	// After completing all turns, set the final world and alive cell count in the response
	res.World = world
	res.Turn = fTurn
	return nil
}

// RPC wrapper for CalculateNextState, so the client can call broker.CalculateNextState
func (b *Broker) CalculateNextState(req stubs.Request, res *stubs.Response) error {
	numServers := len(b.servers)
	if numServers == 0 {
		return fmt.Errorf("no servers available to handle the workload")
	}
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
		subWorld := req.World[startY:endY]
		// Prepare request for each server
		subReq := stubs.Request{
			World:      subWorld,
			P:          req.P,
			StartX:     req.StartX,
			EndX:       req.EndX,
			StartY:     startY,
			EndY:       endY,
			Turns:      req.Turns,
			ServerAddr: serverAddrs,
			TurnDoing:  req.TurnDoing,
		}
		go func(i int) {
			errCh <- b.servers[i].Call("GOL.CalculateNextState", subReq, &responses[i])
		}(i)
		startY = endY
	}

	// Collect responses
	var hasErrors bool
	for i := 0; i < numServers; i++ {
		if err := <-errCh; err != nil {
			hasErrors = true
			//return fmt.Errorf("error from server %d: %v", i, err)
		}
	}
	if hasErrors {
		b.healthCheck()
		time.Sleep(10 * time.Millisecond)
		if len(b.servers) > 0 {
			fmt.Println("Redistributing workload among remaining servers...")
			return b.CalculateNextState(req, res) // Retry with updated server list
		}
		fmt.Println("all servers are unavailable or disconnected")
		return nil
	}

	// Aggregate results from responses
	mu.Lock()
	startY = req.StartY
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
	mu.Unlock()
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
	if numServers == 0 {
		return fmt.Errorf("no servers available to handle the workload")
	}
	heightPerServer := (req.EndY - req.StartY) / numServers
	extraHeight := (req.EndY - req.StartY) % numServers
	responses := make([]stubs.Response, numServers)
	errCh := make(chan error, numServers)

	startY := req.StartY
	// Send each part to a different server
	for i := 0; i < numServers; i++ {
		endY := startY + heightPerServer
		if extraHeight > i {
			endY++
		}

		// Prepare request for each server
		subReq := stubs.Request{
			World:  req.World,
			P:      req.P,
			StartX: req.StartX,
			EndX:   req.EndX,
			StartY: startY,
			EndY:   endY,
			Turns:  req.Turns,
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

	res.FlippedCells = func(responses []stubs.Response) []util.Cell {
		var fc []util.Cell
		for _, s := range responses {
			fc = append(fc, s.FlippedCells...)
		}
		return fc
	}(responses)
	return nil
}

func (b *Broker) Pause(req stubs.Request, res *stubs.Response) error {
	mu.Lock()
	b.paused = true
	fmt.Println("Broker paused")
	mu.Unlock()
	return nil
}

func (b *Broker) Resume(req stubs.Request, res *stubs.Response) error {
	mu.Lock()
	b.paused = false
	b.pausedCond.Broadcast() // Wake up any waiting computations
	fmt.Println("Broker resumed")
	mu.Unlock()
	return nil
}

func (b *Broker) CloseMachine(req stubs.Request, res *stubs.Response) error {
	b.closedLM = true
	return nil
}

func (b *Broker) ClosingSystem(req stubs.Request, res *stubs.Response) error {
	fmt.Println("Closing all servers and broker...")

	// Close each server connection by calling their CloseSystem method
	resClose := new(stubs.Response)
	for _, server := range b.servers {
		err := server.Call("GOL.ClosingSystem", req, resClose)
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

var serverAddrs []string

func main() {
	//serversFlag := flag.String("servers", "", "Comma-separated list of server addresses")
	brokerAddr := flag.String("brokerAddr", "127.0.0.1:8050", "Broker address for client to connect")
	flag.Parse()
	//serverAddrs = strings.Split(*serversFlag, ",")
	// Connect broker to the server
	broker, err := NewBroker()
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
