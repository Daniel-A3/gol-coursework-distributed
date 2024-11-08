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

func closeServer(client *rpc.Client) {
	request := Request{Terminate: true}
	response := new(Response)
	err := client.Call(closingSystemB, request, response)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func processTurnsCall(client *rpc.Client, p Params, world [][]byte, startX, endX, startY, endY, turn int, c distributorChannels) ([][]byte, int) {
	request := Request{World: world, P: p, StartX: startX, EndX: endX, StartY: startY, EndY: endY, Turn: turn}
	response := new(Response)
	err := client.Call(calculateNextStateB, request, response)
	if err != nil {
		fmt.Println(err)
		return world, 1
	}
	c.events <- CellsFlipped{turn, response.FlippedCells}
	return response.World, 0
}

func sendFinalState(client *rpc.Client, p Params, world [][]byte, c distributorChannels, turn int) {
	reqA := Request{World: world, P: p, StartX: 0, EndX: p.ImageWidth, StartY: 0, EndY: p.ImageHeight, Turn: turn}
	resA := new(Response)

	finalAliveCells := make([]util.Cell, p.ImageWidth*p.ImageHeight)
	client.Call(calculateAliveCellsB, reqA, resA)
	finalAliveCells = resA.FlippedCells
	finalState := FinalTurnComplete{turn, finalAliveCells}
	c.events <- finalState
}

var broker = flag.String("server", "127.0.0.1:8050", "IP:port string to connect to as server")

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	var mu sync.Mutex
	cond := sync.NewCond(&mu)

	flag.Parse()
	client, _ := rpc.Dial("tcp", *broker)
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(client)

	// TODO: Create a 2D slice to store the world.
	world := createWorld(p.ImageHeight, p.ImageWidth)

	// Gets the world from input
	world = inputToWorld(p, world, c)

	// List of channels for workers with the size of the amount of threads that are going to be used
	//channels := make([]chan [][]byte, p.Threads)
	turn := 0
	gamePaused := false
	quit := false
	quitS := false

	c.events <- StateChange{turn, Executing}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	go func() {
		resT := new(Response)
		for range ticker.C {
			mu.Lock()
			if turn != 0 && !gamePaused && !quit {
				reqT := Request{World: world, P: p, StartX: 0, EndX: p.ImageWidth, StartY: 0, EndY: p.ImageHeight}
				client.Call(calculateAliveCellsB, reqT, resT)
				c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: resT.Alive}
			}
			mu.Unlock()
		}
	}()

	// Recognising key presses
	go func() {
		for {
			select {
			case key := <-c.keyPresses:
				switch key {
				case 's':
					mu.Lock()
					// Puts current world into PMG file
					worldToOutput(p, world, c, turn)
					mu.Unlock()
				case 'p':
					if !gamePaused {
						mu.Lock()
						gamePaused = true
						fmt.Println("Paused")
						c.events <- StateChange{turn, Paused}
						mu.Unlock()
					} else {
						mu.Lock()
						gamePaused = false

						cond.Broadcast() // Resume all paused workers

						fmt.Println("Resumed")
						c.events <- StateChange{turn, Executing}
						mu.Unlock()
					}
				case 'q':
					mu.Lock()
					ticker.Stop()
					quit = true
					if gamePaused {
						gamePaused = !gamePaused
						cond.Broadcast()
					}
					mu.Unlock()
					return
				case 'k':
					mu.Lock()
					ticker.Stop()
					quit = true
					quitS = true
					if gamePaused {
						gamePaused = !gamePaused
						cond.Broadcast()
					}
					mu.Unlock()
					return
				}

			}
		}
	}()
	var errC int
	for turn < p.Turns {
		mu.Lock()
		for gamePaused {
			cond.Wait()
		}
		mu.Unlock()
		mu.Lock()
		world, errC = processTurnsCall(client, p, world, 0, p.ImageWidth, 0, p.ImageHeight, turn+1, c)
		turn++
		if errC == 1 {
			turn--
			terminate(client, p, world, c, turn, quitS)
			close(c.events)
			return
		}
		c.events <- TurnComplete{turn}
		if quit {
			terminate(client, p, world, c, turn, quitS)
			close(c.events)
			return
		}
		mu.Unlock()
	}
	sendFinalState(client, p, world, c, turn)
	worldToOutput(p, world, c, turn)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	quit = true
	ticker.Stop()
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

func terminate(client *rpc.Client, p Params, world [][]byte, c distributorChannels, turn int, quitS bool) {
	sendFinalState(client, p, world, c, turn)
	// Outputs the final world into a pmg file
	worldToOutput(p, world, c, turn)
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- StateChange{turn, Quitting}
	if quitS {
		closeServer(client)
	}
}
