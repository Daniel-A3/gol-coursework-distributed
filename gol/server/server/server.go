package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol/server/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GOL struct{}

func (gol *GOL) CalculateNextState(req stubs.Request, res *stubs.Response) error {
	height := req.EndY - req.StartY
	width := req.EndX - req.StartX
	nextWorld := createWorld(height, width)
	var cF []util.Cell

	numWorkers := height
	if numWorkers > 16 {
		numWorkers = 16
	}
	if numWorkers < 4 {
		numWorkers = 4
	}

	rowsPerWorker := height / numWorkers
	extraRows := height % numWorkers

	var wg sync.WaitGroup
	flippedCells := make([][]util.Cell, numWorkers) // Slice to store flipped cells from each worker
	worlds := make([][][]byte, numWorkers)          // Slice to store partial nextWorlds from each worker

	for w := 0; w < numWorkers; w++ {
		startY := w * rowsPerWorker
		endY := startY + rowsPerWorker
		if w < extraRows {
			endY++
		}

		wg.Add(1)
		go func(workerIndex, startY, endY int) {
			defer wg.Done()
			flipped, partialWorld := calculateNextState(req.P, req.World, req.StartX, req.EndX, startY, endY)
			flippedCells[workerIndex] = flipped
			worlds[workerIndex] = partialWorld
		}(w, startY, endY)
	}

	wg.Wait()

	// Combine results from workers
	for _, flipped := range flippedCells {
		cF = append(cF, flipped...)
	}

	// Merge `worlds` back into `nextWorld`
	startY := 0
	for _, partialWorld := range worlds {
		for i := range partialWorld {
			copy(nextWorld[startY+i], partialWorld[i])
		}
		startY += len(partialWorld)
	}

	res.FlippedCells = cF
	res.World = nextWorld
	return nil
}

func calculateNextState(p stubs.Params, world [][]byte, startX, endX, startY, endY int) ([]util.Cell, [][]byte) {
	height := endY - startY
	width := endX - startX
	nextWorld := createWorld(height, width)
	var cellsFlipped []util.Cell

	countAlive := func(y, x int) int {
		alive := 0
		for i := -1; i <= 1; i++ {
			for j := -1; j <= 1; j++ {
				if i == 0 && j == 0 {
					continue // Skip the cell itself
				}
				neighbourY := (y + i + p.ImageHeight) % p.ImageHeight
				neighbourX := (x + j + p.ImageWidth) % p.ImageWidth
				if world[neighbourY][neighbourX] == 255 {
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
					cellsFlipped = append(cellsFlipped, util.Cell{X: x, Y: y})
				} else {
					nextWorld[y-startY][x] = 255
				}
			} else {
				if aliveNeighbour == 3 {
					nextWorld[y-startY][x] = 255
					cellsFlipped = append(cellsFlipped, util.Cell{X: x, Y: y})
				} else {
					nextWorld[y-startY][x] = 0
				}
			}
		}
	}
	return cellsFlipped, nextWorld
}

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

func createWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

var closingServer = make(chan struct{})

func (gol *GOL) ClosingSystem(req stubs.Request, response *stubs.Response) error {
	close(closingServer)
	return nil
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Register(&GOL{})
	go func(listener net.Listener) {
		<-closingServer
		listener.Close()
	}(listener)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Closing server...")
			break
		}
		rpc.ServeConn(conn)
	}
}
