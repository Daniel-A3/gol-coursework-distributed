package gol

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

type EventReceiver struct {
	c distributorChannels
}

var gamePaused = false
var quitEverything = false
var turn = 0

func (receiver *EventReceiver) TurnCompleteEvent(req RequestEvent, response *ResponseEvent) error {
	receiver.c.events <- CellsFlipped{Cells: req.FlippedCells, CompletedTurns: req.TurnDone}
	receiver.c.events <- TurnComplete{CompletedTurns: req.TurnDone}
	turn = req.TurnDone
	return nil
}

func retrieveWorld(client *rpc.Client) ([][]byte, int) {
	req := Request{}
	res := new(Response)
	client.Call(sendWorld, req, res)
	return res.World, res.Turn
}

func sendFinalState(client *rpc.Client, p Params, world [][]byte, c distributorChannels, turn int) {
	reqA := Request{World: world, P: p, StartX: 0, EndX: p.ImageWidth, StartY: 0, EndY: p.ImageHeight, Turns: turn}
	resA := new(Response)

	finalAliveCells := make([]util.Cell, p.ImageWidth*p.ImageHeight)
	client.Call(calculateAliveCellsB, reqA, resA)
	finalAliveCells = resA.FlippedCells
	finalState := FinalTurnComplete{turn, finalAliveCells}
	c.events <- finalState
}

var broker = flag.String("broker", "127.0.0.1:8050", "IP:port string to connect to broker")
var localAddr = flag.String("localAddr", "127.0.0.1:8060", "IP:port localAddr")

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	var mu sync.Mutex
	//cond := sync.NewCond(&mu)

	flag.Parse()
	client, _ := rpc.Dial("tcp", *broker)
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(client)

	receiver := &EventReceiver{c: c}
	server := rpc.NewServer()
	err := server.Register(receiver)
	if err != nil {
		fmt.Printf("failed to register event receiver: %v", err)
		os.Exit(1)
	}

	addr := *localAddr
	listener, err := net.Listen("tcp", addr)
	defer listener.Close()
	if err != nil {
		fmt.Printf("failed to listen on %s: %v", addr, err)
		os.Exit(1)
	}
	go server.Accept(listener)
	reqCallback := RequestEvent{CallbackAddr: *localAddr}
	resCallback := new(ResponseEvent)
	err = client.Call("Broker.RegisterCallback", reqCallback, &resCallback)
	if err != nil {
		fmt.Println("Failed to register callback with broker:", err)
		return
	}

	// TODO: Create a 2D slice to store the world.
	world := createWorld(p.ImageHeight, p.ImageWidth)

	// Gets the world from input
	world = inputToWorld(p, world, c)

	c.events <- StateChange{0, Executing}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	go func() {
		resT := new(Response)
		for range ticker.C {
			mu.Lock()
			reqT := Request{P: p, StartX: 0, EndX: p.ImageWidth, StartY: 0, EndY: p.ImageHeight}
			client.Call(TickerB, reqT, resT)
			if resT.Turn != 0 && !gamePaused {
				c.events <- AliveCellsCount{CompletedTurns: resT.Turn, CellsCount: resT.Alive}
			}
			mu.Unlock()

		}
	}()

	//Recognising key presses
	go func() {
		for {
			select {
			case key := <-c.keyPresses:
				switch key {
				case 's':
					mu.Lock()
					// Puts current world into PMG file
					sWorld, sTurn := retrieveWorld(client)
					worldToOutput(p, sWorld, c, sTurn)
					mu.Unlock()
				case 'p':
					req := Request{}
					res := new(Response)
					if !gamePaused {
						mu.Lock()
						gamePaused = true
						fmt.Println("Paused")
						client.Call(pause, req, res)
						c.events <- StateChange{turn, Paused}
						mu.Unlock()
					} else {
						mu.Lock()
						gamePaused = false
						client.Call(resume, req, res)
						fmt.Println("Resumed")
						c.events <- StateChange{turn, Executing}
						mu.Unlock()
					}
				case 'q':
					mu.Lock()
					req := Request{}
					res := new(Response)
					ticker.Stop()
					client.Call(closeLM, req, res)
					if gamePaused {
						gamePaused = !gamePaused
						client.Call(resume, req, res)
					}
					mu.Unlock()
					return
				case 'k':
					mu.Lock()
					req := Request{}
					res := new(Response)
					ticker.Stop()
					quitEverything = true
					client.Call(closeLM, req, res)
					if gamePaused {
						gamePaused = !gamePaused
						client.Call(resume, req, res)
					}
					mu.Unlock()
					return
				}

			}
		}
	}()
	req := Request{
		World:  world,
		P:      p,
		StartX: 0,
		EndX:   p.ImageWidth,
		StartY: 0,
		EndY:   p.ImageHeight,
		Turns:  p.Turns,
	}
	res := new(Response)
	client.Call(calcutateTurnsB, req, res)
	finalTurn := res.Turn
	world = res.World
	sendFinalState(client, p, world, c, finalTurn)
	worldToOutput(p, world, c, finalTurn)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{res.Turn, Quitting}
	if quitEverything {
		reqClose := Request{}
		resClose := new(Response)
		client.Call(closingSystemB, reqClose, resClose)
	}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	//ticker.Stop()
	close(c.events)
}

func inputToWorld(p Params, world [][]byte, c distributorChannels) [][]byte {
	// Read the file
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput
			if world[y][x] == 255 {
				c.events <- CellFlipped{0, util.Cell{X: x, Y: y}}
			}
		}
	}

	return world
}

func createWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func worldToOutput(p Params, world [][]byte, c distributorChannels, turn int) {
	// Outputs the final world into a pmg file
	c.ioCommand <- ioOutput
	fileName := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)
	c.ioFilename <- fileName
	// sends each cell into output channel
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- ImageOutputComplete{turn, fileName}
}
