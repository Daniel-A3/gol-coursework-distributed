package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/gol/server/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GOL struct {
	upperRow []byte
	lowerRow []byte
	muRows   sync.Mutex
	turn     int
}

var genError bool
var clientPool = make(map[string]*rpc.Client)
var muClientPool sync.Mutex

func (gol *GOL) getHalo(addrs []string, halo [][]byte) [][]byte {
	var index int
	for i, addr := range addrs {
		if addr == address {
			index = i
			break
		}
	}
	if len(addrs) == 1 {
		halo[0] = gol.lowerRow
		halo[1] = gol.upperRow
		return halo
	}

	aboveIndex := (index - 1 + len(addrs)) % len(addrs)
	belowIndex := (index + 1) % len(addrs)

	halo[0] = gol.fetchWithRetry(addrs[aboveIndex], true, 3)  // Fetch above row
	halo[1] = gol.fetchWithRetry(addrs[belowIndex], false, 3) // Fetch below row

	if halo[0] == nil || halo[1] == nil {
		fmt.Println("FAIL HALO")
		genError = true
		return halo
	}

	return halo
}

func (gol *GOL) fetchHaloRow(addr string, above bool) []byte {
	client := getClient(addr)
	if client == nil {
		return nil
	}

	req := stubs.ServerRequest{Above: above, TurnDoing: gol.turn}
	res := new(stubs.ServerResponse)
	if err := client.Call("GOL.SendRow", req, res); err != nil {
		fmt.Println("Error in RPC call:", err)
		return nil
	}
	return res.Row
}
func (gol *GOL) fetchWithRetry(addr string, above bool, maxRetries int) []byte {
	for i := 0; i < maxRetries; i++ {
		row := gol.fetchHaloRow(addr, above)
		if row != nil {
			return row
		}
		time.Sleep(10 * time.Millisecond) // Small delay before retrying
	}
	fmt.Println("Failed to fetch halo row after retries")
	return nil
}

func getClient(addr string) *rpc.Client {
	muClientPool.Lock()
	defer muClientPool.Unlock()

	if client, exists := clientPool[addr]; exists {
		return client
	}

	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		fmt.Println("Error dialing:", err)
		return nil
	}

	clientPool[addr] = client
	return client
}

func (gol *GOL) SendRow(req stubs.ServerRequest, res *stubs.ServerResponse) error {
	gol.muRows.Lock()
	defer gol.muRows.Unlock()
	if req.TurnDoing == gol.turn {
		if req.Above == true {
			res.Row = gol.lowerRow
		} else {
			res.Row = gol.upperRow
		}
	}

	return nil
}

var muNextState sync.Mutex

func (gol *GOL) CalculateNextState(req stubs.Request, res *stubs.Response) error {
	gol.muRows.Lock()
	gol.upperRow = req.World[0]
	gol.lowerRow = req.World[len(req.World)-1]
	gol.turn = req.TurnDoing
	gol.muRows.Unlock()

	height := req.EndY - req.StartY
	width := req.EndX - req.StartX

	nextWorld := createWorld(height, width)

	haloWorld := make([][]byte, height+2)
	halo := make([][]byte, 2)
	halo = gol.getHalo(req.ServerAddr, halo)
	if genError {
		return fmt.Errorf("error occured")
	}
	muNextState.Lock()
	haloWorld[0] = halo[0]
	for i := 0; i < height; i++ {
		haloWorld[i+1] = req.World[i]
	}
	haloWorld[len(haloWorld)-1] = halo[1]
	muNextState.Unlock()

	var cF []util.Cell

	numWorkers := height
	if numWorkers > 16 {
		numWorkers = 16
	}

	rowsPerWorker := height / numWorkers
	extraRows := height % numWorkers

	var wg sync.WaitGroup
	flippedCells := make([][]util.Cell, numWorkers) // Slice to store flipped cells from each worker
	worlds := make([][][]byte, numWorkers)          // Slice to store partial nextWorlds from each worker
	startY := req.StartY
	for w := 0; w < numWorkers; w++ {
		endY := startY + rowsPerWorker
		if w < extraRows {
			endY++
		}

		wg.Add(1)
		go func(workerIndex, startY, endY int) {
			defer wg.Done()
			flipped, partialWorld := calculateNextState(haloWorld, req.StartX, req.EndX, startY, endY, req.StartY)
			flippedCells[workerIndex] = flipped
			worlds[workerIndex] = partialWorld
		}(w, startY, endY)
		startY = endY
	}

	wg.Wait()
	muNextState.Lock()
	defer muNextState.Unlock()
	// Combine results from workers
	for _, flipped := range flippedCells {
		cF = append(cF, flipped...)
	}

	// Merge `worlds` back into `nextWorld`
	startY = 0
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

func calculateNextState(world [][]byte, startX, endX, startY, endY, serverY int) ([]util.Cell, [][]byte) {
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
				neighbourY := y + i
				neighbourX := (x + j + width) % width
				if world[neighbourY][neighbourX] == 255 {
					alive++
				}
			}
		}
		return alive
	}

	for y := 1; y < height+1; y++ {
		for x := startX; x < endX; x++ {
			aliveNeighbour := countAlive(startY+y-serverY, x)
			if world[y+startY-serverY][x] == 255 { // Cell is alive
				if aliveNeighbour < 2 || aliveNeighbour > 3 {
					nextWorld[y-1][x] = 0 // Cell dies
					cellsFlipped = append(cellsFlipped, util.Cell{X: x, Y: y - 1 + startY})
				} else {
					nextWorld[y-1][x] = 255 // Cell stays alive
				}
			} else { // Cell is dead
				if aliveNeighbour == 3 {
					nextWorld[y-1][x] = 255 // Cell becomes alive
					cellsFlipped = append(cellsFlipped, util.Cell{X: x, Y: y - 1 + startY})
				} else {
					nextWorld[y-1][x] = 0 // Cell remains dead
				}
			}
		}
	}

	return cellsFlipped, nextWorld
}

func (gol *GOL) CalculateAliveCells(req stubs.Request, res *stubs.Response) error {
	numWorkers := req.EndY - req.StartY
	if numWorkers > 16 {
		numWorkers = 16
	}

	rowsPerWorker := (req.EndY - req.StartY) / numWorkers
	extraRows := (req.EndY - req.StartY) % numWorkers

	var wg sync.WaitGroup
	aliveCounts := make([]int, numWorkers)        // Slice to store alive counts from each worker
	aliveCells := make([][]util.Cell, numWorkers) // Slice to store alive cells from each worker

	startY := req.StartY
	for w := 0; w < numWorkers; w++ {
		endY := startY + rowsPerWorker
		if w < extraRows {
			endY++
		}

		wg.Add(1)
		go func(workerIndex, startY, endY int) {
			defer wg.Done()
			count := 0
			var alive []util.Cell
			for y := startY; y < endY; y++ {
				for x := req.StartX; x < req.EndX; x++ {
					if req.World[y][x] == 255 {
						count++
						alive = append(alive, util.Cell{X: x, Y: y})
					}
				}
			}
			aliveCounts[workerIndex] = count
			aliveCells[workerIndex] = alive
		}(w, startY, endY)

		startY = endY
	}

	wg.Wait()

	// Combine results from workers
	totalAlive := 0
	for _, count := range aliveCounts {
		totalAlive += count
	}
	res.Alive = totalAlive

	// Combine flipped cells
	for _, cells := range aliveCells {
		res.FlippedCells = append(res.FlippedCells, cells...)
	}
	res.World = req.World // Return the original world as is
	return nil
}

func (gol *GOL) Ping(req stubs.Request, res *stubs.Response) error {
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
func closeClients() {
	muClientPool.Lock()
	defer muClientPool.Unlock()

	for addr, client := range clientPool {
		if err := client.Close(); err != nil {
			fmt.Printf("Error closing client for %s: %v\n", addr, err)
		} else {
			fmt.Printf("Closed client for %s\n", addr)
		}
		delete(clientPool, addr) // Remove from the pool after closing
	}
}

var address string
var broker = flag.String("broker", "127.0.0.1:8050", "IP:port string to connect to broker")

func main() {
	port := flag.String("port", "8030", "Addr for broker to connect to")
	ipAddr := flag.String("ip", "127.0.0.1", "IP address to listen on")
	flag.Parse()
	address = fmt.Sprintf("%s:%s", *ipAddr, *port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	fmt.Printf("Listening on :%s\n", address)
	defer listener.Close()
	rpc.Register(&GOL{})

	client, _ := rpc.Dial("tcp", *broker)
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(client)
	req := stubs.BrokerRequest{Addr: address}
	res := new(stubs.BrokerResponse)
	client.Call("Broker.AddServer", req, res)
	go func(listener net.Listener) {
		<-closingServer
		listener.Close()
	}(listener)
	defer closeClients()
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Closing server...")
			break
		}
		go rpc.ServeConn(conn)
	}
}
