package tictactoe

import "errors"

func init() {
	NewGame()
}

type Game struct {
	Id            int
	Board         Board
	Player1       string
	Player2       string
	Winner        string
	currentPlayer string
	spectators    []string
}

var count = 0

var Games = []*Game{}

func NewGame() *Game {
	count++
	game := &Game{
		Id:    count,
		Board: Board{},
	}
	Games = append(Games, game)
	return game
}

func (g *Game) Info() string {
	if g.Winner != "" {
		return "Player " + g.Winner + " wins!"
	}

	if g.Player1 == "" && g.Player2 == "" {
		return "Waiting for players"
	} else if g.Player2 == "" {
		return "Waiting for player 2"
	}

	return "Playing " + g.Player1 + " vs " + g.Player2
}

func (g *Game) PlayStatus() string {
	if g.Winner != "" {
		return "Game over! " + g.Winner + " wins!"
	}

	if g.Player1 == "" {
		return "Waiting for players"
	} else if g.Player2 == "" {
		return "Waiting for player 2"
	}

	displayName := "Player 1"
	if g.currentPlayer == g.Player2 {
		displayName = "Player 2"
	}
	return "Current player: " + displayName
}

func (g *Game) Join(player string) {
	if g.Player1 == player || g.Player2 == player {
		return
	}

	if g.Player1 == "" {
		g.Player1 = player
	} else if g.Player2 == "" {
		g.Player2 = player
		g.currentPlayer = g.Player1
	} else {
		g.spectators = append(g.spectators, player)
	}
}

func (g *Game) PlayMove(player int, index int) error {
	if g.Winner != "" {
		return errors.New("The game has already ended")
	}
	if g.currentPlayer == "" {
		return errors.New("Game has not started yet")
	}
	if g.Board.GetCell(index) != 0b00 {
		return errors.New("Cell not empty")
	}
	if (player == 1 && g.currentPlayer != g.Player1) || (player == 2 && g.currentPlayer != g.Player2) {
		return errors.New("Not your turn")
	}
	if g.BoardFull() {
		return errors.New("The board is full")
	}

	g.Board.setCell(index, player)

	if g.CheckWinner() {
		g.Winner = g.currentPlayer
		return nil
	}

	if g.currentPlayer == g.Player1 {
		g.currentPlayer = g.Player2
	} else {
		g.currentPlayer = g.Player1
	}
	return nil
}

func (g *Game) GetCell(index int) *Cell {
	return &Cell{
		Symbol: g.Board.Symbol(uint(index)),
		Index:  uint(index),
		GameId: g.Id,
	}
}

func (g *Game) BoardFull() bool {
	for i := 0; i < 9; i++ {
		if g.Board.GetCell(i) == 0b00 {
			return false
		}
	}
	return true
}

func (g *Game) CheckWinner() bool {

	// Horizontal
	for i := 0; i < 9; i++ {
		row := g.Board.GetCell(i) & g.Board.GetCell(i+1) & g.Board.GetCell(i+2)
		if row == 0b01 || row == 0b10 {
			return true
		}
	}

	// Vertical
	for i := 0; i < 3; i++ {
		column := g.Board.GetCell(i) & g.Board.GetCell(i+3) & g.Board.GetCell(i+6)
		if column == 0b01 || column == 0b10 {
			return true
		}
	}

	// Diagonal
	diagonal1 := g.Board.GetCell(0) & g.Board.GetCell(4) & g.Board.GetCell(8)
	if diagonal1 == 0b01 || diagonal1 == 0b10 {
		return true
	}

	diagonal2 := g.Board.GetCell(2) & g.Board.GetCell(4) & g.Board.GetCell(6)
	if diagonal2 == 0b01 || diagonal2 == 0b10 {
		return true
	}

	return false
}
