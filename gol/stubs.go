package gol

import "uk.ac.bris.cs/gameoflife/util"

var calculateNextState = "GOL.CalculateNextState"
var calculateAliveCells = "GOL.CalculateAliveCells"
var closingSystem = "GOL.ClosingSystem"

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
