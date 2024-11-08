package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/gol/server/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GOL struct{}

// CalculateNextState processes the next state of the Game of Life grid.
func (gol *GOL) CalculateNextState(req stubs.Request, res *stubs.Response) error {
	height := req.EndY - req.StartY
	width := req.EndX - req.StartX
	nextWorld := createWorld(height, width)
	var cellsFlipped []util.Cell

	countAlive := func(y, x int) int {
		alive := 0
		for i := -1; i <= 1; i++ {
			for j := -1; j <= 1; j++ {
				neighbourY := (y + i + req.P.ImageHeight) % req.P.ImageHeight
				neighbourX := (x + j + req.P.ImageWidth) % req.P.ImageWidth
				if !(i == 0 && j == 0) && (req.World[neighbourY][neighbourX] == 255) {
					alive++
				}
			}
		}
		return alive
	}

	for y := req.StartY; y < req.EndY; y++ {
		for x := req.StartX; x < req.EndX; x++ {
			aliveNeighbour := countAlive(y, x)

			if req.World[y][x] == 255 { // Cell is alive
				if aliveNeighbour < 2 || aliveNeighbour > 3 {
					nextWorld[y-req.StartY][x] = 0 // Cell dies
					cellsFlipped = append(cellsFlipped, util.Cell{X: x, Y: y})
				} else {
					nextWorld[y-req.StartY][x] = 255 // Cell stays alive
				}
			} else { // Cell is dead
				if aliveNeighbour == 3 {
					nextWorld[y-req.StartY][x] = 255 // Cell becomes alive
					cellsFlipped = append(cellsFlipped, util.Cell{X: x, Y: y})
				} else {
					nextWorld[y-req.StartY][x] = 0 // Cell remains dead
				}
			}
		}
	}
	res.FlippedCells = cellsFlipped
	res.World = nextWorld
	return nil
}

// CalculateAliveCells counts the alive cells in the grid.
func (gol *GOL) CalculateAliveCells(req stubs.Request, res *stubs.Response) error {
	var alive []util.Cell
	count := 0
	for y := req.StartY; y < req.EndY; y++ {
		for x := req.StartX; x < req.EndX; x++ {
			if req.World[y][x] == 255 {
				count++
				alive = append(alive, util.Cell{X: x, Y: y})
			}
		}
	}
	res.Alive = count
	res.FlippedCells = alive
	res.World = req.World
	return nil
}

// createWorld initializes a new world grid.
func createWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

var closingServer = make(chan struct{})

// ClosingSystem handles server shutdown.
func (gol *GOL) ClosingSystem(req stubs.Request, response *stubs.Response) error {
	close(closingServer)
	return nil
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	rpc.Register(&GOL{})
	go func(listener net.Listener) {
		<-closingServer
		listener.Close()
	}(listener)

	for {
		// Accept connections until the listener is closed
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Closing server...")
			break
		}
		rpc.ServeConn(conn)
	}
}
