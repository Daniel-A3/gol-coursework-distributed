package stubs

import "uk.ac.bris.cs/gameoflife/util"

var calculateNextState = "GOL.DistributeNext"
var calculateAliveCells = "GOL.DistributeAlive"
var closingSystem = "GOL.ClosingSystem"
var ping = "GOL.Ping"
var calculateNextStateB = "Broker.CalculateNextState"
var calculateAliveCellsB = "Broker.CalculateAliveCells"
var closingSystemB = "Broker.ClosingSystem"
var calcutateTurnsB = "Broker.CalculateTurns"
var notify = "EventReceiver.TurnCompleteEvent"

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
	World  [][]byte
	P      Params
	StartX int
	EndX   int
	StartY int
	EndY   int
	Turns  int
}

type RequestEvent struct {
	TurnDone     int
	FlippedCells []util.Cell
	Alive        int
	CallbackAddr string
}

type ResponseEvent struct {
}
