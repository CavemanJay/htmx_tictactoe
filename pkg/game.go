package tictactoe

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

func (g *Game) GetCell(index int) *Cell {
	return &Cell{
		Symbol: g.Board.Symbol(uint(index)),
		Index:  uint(index),
		GameId: g.Id,
	}
}
