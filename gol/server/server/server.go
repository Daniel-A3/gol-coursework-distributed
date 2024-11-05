package main

import (
	//	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"uk.ac.bris.cs/gameoflife/gol/server/stubs"
	"uk.ac.bris.cs/gameoflife/util"
	//	"time"
	//	"math/rand"
	"net/rpc"
)

type GOL struct {
}

func (gol *GOL) CalculateNextState(req stubs.Request, res *stubs.Response) error {
	height := req.EndY - req.StartY
	width := req.EndX - req.StartX
	nextWorld := createWorld(height, width)
	var cellsFlipped []util.Cell

	countAlive := func(y, x int) int {
		alive := 0
		for i := -1; i < 2; i++ {
			for j := -1; j < 2; j++ {
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

			if req.World[y][x] == 255 {
				if aliveNeighbour < 2 || aliveNeighbour > 3 {
					nextWorld[y-req.StartY][x] = 0
					cellsFlipped = append(cellsFlipped, util.Cell{X: x, Y: y})
				} else {
					nextWorld[y-req.StartY][x] = 255
				}
			} else {
				if aliveNeighbour == 3 {
					nextWorld[y-req.StartY][x] = 255
					cellsFlipped = append(cellsFlipped, util.Cell{X: x, Y: y})
				} else {
					nextWorld[y-req.StartY][x] = 0
				}
			}
		}
	}
	res.FlippedCells = cellsFlipped
	res.World = nextWorld
	return nil
}

func (gol *GOL) CalculateAliveCells(req stubs.Request, res *stubs.Response) error {
	var alive []util.Cell
	count := 0
	for y := 0; y < req.P.ImageHeight; y++ {
		for x := 0; x < req.P.ImageWidth; x++ {
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

func createWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

var closingServer = false

func (gol *GOL) ClosingSystem(req stubs.Request, response *stubs.Response) error {
	closingServer = true
	return nil
}
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	closing := false
	go func() {
		for !closing {
			if closingServer {
				closing = true
				os.Exit(0)
			}
		}
	}()
	rpc.Register(&GOL{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
	fmt.Println(listener)

}
