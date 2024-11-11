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
var sendWorld = "Broker.SendWorld"
var pause = "Broker.Pause"
var resume = "Broker.Resume"
var closeLM = "Broker.CloseMachine"

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

type RequestEvent struct {
	TurnDone     int
	FlippedCells []util.Cell
	Alive        int
	CallbackAddr string
}

type ResponseEvent struct {
}

type ServerRequest struct {
	Above bool
}

type ServerResponse struct {
	Row []byte
}
