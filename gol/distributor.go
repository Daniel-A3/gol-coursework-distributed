package gol

import (
	"flag"
	"fmt"
	"net/rpc"
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

func processTurnsCall(client *rpc.Client, p Params, world [][]byte, startX, endX, startY, endY, turn int) [][]byte {
	request := Request{World: world, P: p, StartX: startX, EndX: endX, StartY: startY, EndY: endY, Turn: turn}
	response := new(Response)
	err := client.Call(calculateNextState, request, response)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return response.World
}

func sendFinalState(client *rpc.Client, p Params, world [][]byte, c distributorChannels, turn int) {
	reqA := Request{World: world, P: p, StartX: 0, EndX: p.ImageWidth, StartY: 0, EndY: p.ImageHeight, Turn: turn}
	resA := new(Response)
	finalAliveCells := make([]util.Cell, p.ImageWidth*p.ImageHeight)
	client.Call(calculateAliveCells, reqA, resA)
	finalAliveCells = resA.FlippedCells
	finalState := FinalTurnComplete{turn, finalAliveCells}
	c.events <- finalState
}

var server = flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	var mu sync.Mutex
	flag.Parse()
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()
	// TODO: Create a 2D slice to store the world.
	world := createWorld(p.ImageHeight, p.ImageWidth)

	// Gets the world from input
	world = inputToWorld(p, world, c)

	// List of channels for workers with the size of the amount of threads that are going to be used
	//channels := make([]chan [][]byte, p.Threads)
	turn := 0

	c.events <- StateChange{turn, Executing}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	go func() {
		resT := new(Response)
		for range ticker.C {
			if turn != 0 {
				mu.Lock()
				reqT := Request{World: world, P: p}
				client.Call(calculateAliveCells, reqT, resT)
				c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: resT.Alive}
				mu.Unlock()
			}
		}
	}()

	for turn < p.Turns {
		world = processTurnsCall(client, p, world, 0, p.ImageWidth, 0, p.ImageHeight, turn)
		turn++
		c.events <- TurnComplete{turn}
	}

	sendFinalState(client, p, world, c, turn)
	worldToOutput(p, world, c, turn)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
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
