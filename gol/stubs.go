package gol

import "uk.ac.bris.cs/gameoflife/util"

type Response struct {
	world        [][]byte
	flippedCells []util.Cell
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
