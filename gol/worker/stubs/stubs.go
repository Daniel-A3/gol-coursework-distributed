package stubs

import "uk.ac.bris.cs/gameoflife/util"

var processTurnsHandler = "GOL.CalculateNextTurn"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Response struct {
	world        [][]byte
	flippedCells []util.Cell
	alive        int
}

type Request struct {
	world  [][]byte
	p      Params
	startX int
	endX   int
	startY int
	endY   int
	turn   int
}
