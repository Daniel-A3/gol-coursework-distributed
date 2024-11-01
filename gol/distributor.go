package gol

import (
	"flag"
	"fmt"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/util"
)

//import "fmt"

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
	fmt.Println("IM HERE 6")
	err := client.Call(calculateNextState, request, response)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return response.World
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	//var mu sync.Mutex
	fmt.Println("IM HERE 1")
	server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	flag.Parse()
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()
	fmt.Println("IM HERE 2")

	// TODO: Create a 2D slice to store the world.
	world := createWorld(p.ImageHeight, p.ImageWidth)
	fmt.Println("IM HERE 3")

	// Gets the world from input
	world = inputToWorld(p, world, c)

	// List of channels for workers with the size of the amount of threads that are going to be used
	//channels := make([]chan [][]byte, p.Threads)
	turn := 0

	c.events <- StateChange{turn, Executing}
	for turn < p.Turns {
		world = processTurnsCall(client, p, world, 0, p.ImageWidth, 0, p.ImageHeight, turn)
		turn++
		c.events <- TurnComplete{turn}
	}

	// TODO: Execute all turns of the Game of Life.

	// TODO: Report the final state using FinalTurnCompleteEvent.
	req := Request{World: world, P: p, StartX: 0, EndX: p.ImageWidth, StartY: 0, EndY: p.ImageHeight, Turn: turn}
	res := new(Response)
	finalAliveCells := make([]util.Cell, p.ImageWidth*p.ImageHeight)
	err := client.Call(calculateAliveCells, req, res)
	if err != nil {
		fmt.Println(err)
	}
	finalAliveCells = res.FlippedCells
	fmt.Println(finalAliveCells)
	finalState := FinalTurnComplete{turn, finalAliveCells}
	c.events <- finalState
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
	fmt.Println("IM HERE 4")
	c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	fmt.Println("IM HERE 5")
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
