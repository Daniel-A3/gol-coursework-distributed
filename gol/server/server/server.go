package main

import (
	"flag"
	"fmt"
	"net"
<<<<<<< HEAD
=======
	"net/rpc"
>>>>>>> 1dd03dd (works with workers for 1 server)
	"sync"
	"uk.ac.bris.cs/gameoflife/gol/server/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GOL struct{}

var mu sync.Mutex

func (gol *GOL) DistributeNext(req stubs.Request, res *stubs.Response) error {
	height := req.EndY - req.StartY
	numWorkers := 1
	newWorld := createWorld(height, req.EndX)
	var flipped []util.Cell
	if height < 16 {
		numWorkers = height
	} else {
		numWorkers = 16
	}

	channelsF := make([]chan ([]util.Cell), numWorkers)
	channelsW := make([]chan ([][]byte), numWorkers)
	for i := 0; i < numWorkers; i++ {
		channelsF[i] = make(chan []util.Cell)
		channelsW[i] = make(chan [][]byte)
	}

	startY := 0
	extraRows := height % numWorkers
	for w := 0; w < numWorkers; w++ {
		// Rows for current worker
		numRows := height / numWorkers
		// Add a row if there are extra rows for workers than need doing
		if extraRows > w {
			numRows++
		}
		endY := startY + numRows
		// Ensure endY does not exceed the bounds of req.World
		if endY > height {
			endY = height
		}
		go workerNext(req.P, req.World, 0, req.EndX, startY, endY, channelsF[w], channelsW[w])
		startY = endY
	}
	startY = 0
	for j := 0; j < numWorkers; j++ {
		numRows := height
		// Add a row if there are extra rows for workers than need doing
		if extraRows > j {
			numRows++
		}
		endY := startY + numRows
		if endY > len(newWorld) {
			endY = len(newWorld)
		}

		copy(newWorld[startY:endY], <-channelsW[j])
		flipped = append(flipped, <-channelsF[j]...)
		startY = endY
	}
	res.World = newWorld
	res.FlippedCells = flipped
	return nil
}

func (gol *GOL) DistributeAlive(req stubs.Request, res *stubs.Response) error {
	height := req.EndY - req.StartY
	numWorkers := 1
	count := 0
	var flipped []util.Cell
	if height < 16 {
		numWorkers = height
	} else {
		numWorkers = 16
	}
	channelsA := make([]chan int, numWorkers)
	channelsF := make([]chan []util.Cell, numWorkers)
	for i := 0; i < numWorkers; i++ {
		channelsA[i] = make(chan int)
		channelsF[i] = make(chan []util.Cell)
	}

	startY := 0
	extraRows := height % numWorkers
	for w := 0; w < numWorkers; w++ {
		// Rows for current worker
		numRows := height / numWorkers
		// Add a row if there are extra rows for workers than need doing
		if extraRows > w {
			numRows++
		}
		endY := startY + numRows
		// Ensure endY does not exceed the bounds of req.World
		if endY > height {
			endY = height
		}
		go workerAlive(req.P, req.World, 0, req.EndX, startY, endY, channelsA[w], channelsF[w])
		startY = endY
	}
	for j := 0; j < numWorkers; j++ {
		flipped = append(flipped, <-channelsF[j]...)
		count += <-channelsA[j]
	}
	res.Alive = count
	res.FlippedCells = flipped
	res.World = req.World
	return nil
}

func calculateNextState(p stubs.Params, world [][]byte, startX, endX, startY, endY int) ([]util.Cell, [][]byte) {
	height := endY - startY
	width := endX - startX
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
<<<<<<< HEAD
		for i := -1; i < 2; i++ {
			for j := -1; j < 2; j++ {
				neighbourY := (y + i + p.ImageHeight) % p.ImageHeight
				neighbourX := (x + j + p.ImageWidth) % p.ImageWidth
				if !(i == 0 && j == 0) && (world[neighbourY][neighbourX] == 255) {
=======
		for i := -1; i <= 1; i++ {
			for j := -1; j <= 1; j++ {
				if i == 0 && j == 0 {
					continue // Skip the cell itself
				}
				neighbourY := (y + i + p.ImageHeight) % p.ImageHeight
				neighbourX := (x + j + p.ImageWidth) % p.ImageWidth
				if world[neighbourY][neighbourX] == 255 {
>>>>>>> 1dd03dd (works with workers for 1 server)
					alive++
				}
			}
		}
		return alive
	}

	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			aliveNeighbour := countAlive(y, x)
<<<<<<< HEAD

=======
>>>>>>> 1dd03dd (works with workers for 1 server)
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

func calculateAliveCells(p stubs.Params, world [][]byte, startX, endX, startY, endY int) (int, []util.Cell) {
	var flipped []util.Cell
	count := 0
<<<<<<< HEAD
	for y := 0; y < endY; y++ {
		for x := 0; x < endX; x++ {
			if world[y][x] == 255 {
=======
	for y := req.StartY; y < req.EndY; y++ {
		for x := req.StartX; x < req.EndX; x++ {
			if req.World[y][x] == 255 {
>>>>>>> 1dd03dd (works with workers for 1 server)
				count++
				flipped = append(flipped, util.Cell{X: x, Y: y})
			}
		}
	}

	return count, flipped
}

func workerNext(p stubs.Params, world [][]byte, startX, endX, startY, endY int, cF chan []util.Cell, cW chan [][]byte) {
	flipped, world := calculateNextState(p, world, startX, endX, startY, endY)
	cF <- flipped
	cW <- world
}

func workerAlive(p stubs.Params, world [][]byte, startX, endX, startY, endY int, cA chan int, cF chan []util.Cell) {
	alive, flipped := calculateAliveCells(p, world, startX, endX, startY, endY)
	fmt.Println(flipped)
	cA <- alive
	cF <- flipped
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
