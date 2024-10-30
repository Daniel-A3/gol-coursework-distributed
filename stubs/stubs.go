package stubs

import "uk.ac.bris.cs/gameoflife/gol"

var nextStepHandler = "GolOperations.nextStep"

type Response struct {
	World [][]byte
}

type Request struct {
	World [][]byte
	p     gol.Params
	turn  int
}
