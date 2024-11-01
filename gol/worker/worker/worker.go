package main

import (
	//	"errors"
	"flag"
	"net"
	"uk.ac.bris.cs/gameoflife/gol/worker/stubs"
	"uk.ac.bris.cs/gameoflife/util"
	//	"time"
	//	"math/rand"
	"net/rpc"
)

type GOL struct {
}

func (gol *GOL) CalculateNextState(req stubs.Request, res *stubs.Response) (err error) {
	height := req.EndY - req.StartY
	width := req.endX - req.startX
	nextWorld := createWorld(height, width)

	countAlive := func(y, x int) int {
		alive := 0
		for i := -1; i < 2; i++ {
			for j := -1; j < 2; j++ {
				neighbourY := (y + i + req.p.ImageHeight) % req.p.ImageHeight
				neighbourX := (x + j + req.p.ImageWidth) % req.p.ImageWidth
				if !(i == 0 && j == 0) && (req.world[neighbourY][neighbourX] == 255) {
					alive++
				}
			}
		}
		return alive
	}

	for y := req.startY; y < req.endY; y++ {
		for x := req.startX; x < req.endX; x++ {
			aliveNeighbour := countAlive(y, x)

			if req.world[y][x] == 255 {
				if aliveNeighbour < 2 || aliveNeighbour > 3 {
					nextWorld[y-req.startY][x] = 0
				} else {
					nextWorld[y-req.startY][x] = 255
				}
			} else {
				if aliveNeighbour == 3 {
					nextWorld[y-req.startY][x] = 255
				} else {
					nextWorld[y-req.startY][x] = 0
				}
			}
		}
	}
	res.world = nextWorld
	return
}

func (gol *GOL) CalculateAliveCells(req stubs.Request, res *stubs.Response) {
	var alive []util.Cell
	count := 0
	for y := 0; y < req.p.ImageHeight; y++ {
		for x := 0; x < req.p.ImageWidth; x++ {
			if req.world[y][x] == 255 {
				count++
				alive = append(alive, util.Cell{X: x, Y: y})
			}
		}
	}
	res.alive = count
	res.flippedCells = alive
	res.world = req.world
	return
}

//func worker(p Params, world [][]byte, startX, endX, startY, endY int, out chan<- [][]byte, c distributorChannels, turn int) {
//	out <- calculateNextState(p, world, startX, endX, startY, endY, c, turn)
//}

func createWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func main() {

	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	err := rpc.Register(&GOL{})
	if err != nil {
		return
	}
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
