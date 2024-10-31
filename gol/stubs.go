package gol

import "uk.ac.bris.cs/gameoflife/util"

var processTurnsHandler = "GOL.CalculateNextTurn"

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
