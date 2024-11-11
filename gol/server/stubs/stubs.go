package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var calculateNextState = "GOL.CalculateNextState"
var calculateAliveCells = "GOL.CalculateAliveCells"
var closingSystem = "GOL.ClosingSystem"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Response struct {
	World        [][]byte
	FlippedCells []util.Cell
	Alive        int
	Turn         int
}

type Request struct {
	World      [][]byte
	P          Params
	StartX     int
	EndX       int
	StartY     int
	EndY       int
	Turns      int
	ServerAddr []string
}

type ServerRequest struct {
	Above bool
}

type ServerResponse struct {
	Row []byte
}

type BrokerRequest struct {
	Addr string
}
type BrokerResponse struct {
}
