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
	World        [][]byte
	FlippedCells []util.Cell
	Alive        int
}

type Request struct {
	World  [][]byte
	P      Params
	StartX int
	EndX   int
	StartY int
	EndY   int
	Turn   int
}
