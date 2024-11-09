package gol

import "uk.ac.bris.cs/gameoflife/util"

var calculateNextState = "GOL.CalculateNextState"
var calculateAliveCells = "GOL.CalculateAliveCells"
var closingSystem = "GOL.ClosingSystem"
var calculateNextStateB = "Broker.CalculateNextState"
var calculateAliveCellsB = "Broker.CalculateAliveCells"
var closingSystemB = "Broker.ClosingSystem"
var calcutateTurnsB = "Broker.CalculateTurns"
var TickerB = "Broker.Ticker"

type Response struct {
	World        [][]byte
	FlippedCells []util.Cell
	Alive        int
	Turn         int
}

type Request struct {
	World  [][]byte
	P      Params
	StartX int
	EndX   int
	StartY int
	EndY   int
	Turns  int
}
