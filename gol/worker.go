package gol

import (
	//	"errors"
	"flag"
	"uk.ac.bris.cs/gameoflife/util"

	//	"fmt"
	"net"
	//	"time"
	//	"math/rand"
	"net/rpc"
)

func calculateNextState(p Params, world [][]byte, startX, endX, startY, endY int, c distributorChannels, turn int) [][]byte {
	height := endY - startY
	width := endX - startX
	nextWorld := createWorld(height, width)

	countAlive := func(y, x int) int {
		alive := 0
		for i := -1; i < 2; i++ {
			for j := -1; j < 2; j++ {
				neighbourY := (y + i + p.ImageHeight) % p.ImageHeight
				neighbourX := (x + j + p.ImageWidth) % p.ImageWidth
				if !(i == 0 && j == 0) && (world[neighbourY][neighbourX] == 255) {
					alive++
				}
			}
		}
		return alive
	}

	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			aliveNeighbour := countAlive(y, x)

			if world[y][x] == 255 {
				if aliveNeighbour < 2 || aliveNeighbour > 3 {
					nextWorld[y-startY][x] = 0
					c.events <- CellFlipped{turn, util.Cell{X: x, Y: y}}
				} else {
					nextWorld[y-startY][x] = 255
				}
			} else {
				if aliveNeighbour == 3 {
					nextWorld[y-startY][x] = 255
					c.events <- CellFlipped{turn, util.Cell{X: x, Y: y}}
				} else {
					nextWorld[y-startY][x] = 0
				}
			}
		}
	}

	return nextWorld
}

func calculateAliveCells(p Params, world [][]byte) (int, []util.Cell) {
	var alive []util.Cell
	count := 0
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				count++
				alive = append(alive, util.Cell{X: x, Y: y})
			}
		}
	}
	return count, alive
}

func worker(p Params, world [][]byte, startX, endX, startY, endY int, out chan<- [][]byte, c distributorChannels, turn int) {
	out <- calculateNextState(p, world, startX, endX, startY, endY, c, turn)
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
