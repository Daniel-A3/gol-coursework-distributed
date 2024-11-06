package stubs

import "uk.ac.bris.cs/gameoflife/util"

var calculateNextState = "GOL.CalculateNextState"
var calculateAliveCells = "GOL.CalculateAliveCells"
var closingSystem = "GOL.ClosingSystem"
var calculateNextStateB = "Broker.CalculateNextState"
var calculateAliveCellsB = "Broker.CalculateAliveCells"
var closingSystemB = "Broker.ClosingSystem"

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
}

type Request struct {
	World     [][]byte
	P         Params
	StartX    int
	EndX      int
	StartY    int
	EndY      int
	Turn      int
	Terminate bool
}
