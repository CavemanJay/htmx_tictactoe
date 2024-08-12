package tictactoe

import (
	"errors"

	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func init() {
	player1 := &Participant{Id: "t1", Name: "Testing 1", Player: true}
	player2 := &Participant{Id: "t2", Name: "Testing 2", Player: true}
	spectator1 := &Participant{Id: "t3", Name: "Testing 3"}
	game := &Game{
		Id:    1,
		Board: Board{value: 0b010101},
		// Player1: "Testing 1",
		// Player2: "Testing 2",
		Player1:       player1,
		Player2:       player2,
		Winner:        player1,
		currentPlayer: player1,
		History:       []Board{{value: 0b01}, {value: 0b0101}},
		// Spectators: map[string]bool{"Testing 3": false},
		// Participants: orderedmap.New[string, bool](
		// 	orderedmap.WithInitialData[string, bool](orderedmap.Pair[string, bool]{Key: "Testing 3", Value: false})),
		// Participants: orderedmap.New[*Participant, bool](
		// 	orderedmap.WithInitialData[*Participant, bool](orderedmap.Pair[*Participant, bool]{Key: spectator1, Value: false})),
		Participants: orderedmap.New[ParticipantId, *Participant](
			orderedmap.WithInitialData[ParticipantId, *Participant](orderedmap.Pair[ParticipantId, *Participant]{Key: spectator1.Id, Value: spectator1})),
	}

	Games = append(Games, game)
	NewGame()
}

type GameId int
type ParticipantId string

type Participant struct {
	Id        ParticipantId
	Name      string
	Player    bool
	Connected bool
}

type Game struct {
	Id            GameId
	Board         Board
	Player1       *Participant
	Player2       *Participant
	Winner        *Participant
	Participants  *orderedmap.OrderedMap[ParticipantId, *Participant]
	History       []Board
	currentPlayer *Participant
}

var count GameId = 1

var Games = []*Game{}

func NewGame() *Game {
	count++
	game := &Game{
		Id:           count,
		Board:        Board{},
		Participants: orderedmap.New[ParticipantId, *Participant](),
	}
	Games = append(Games, game)
	return game
}

func (g *Game) Info() string {
	if g.Winner != nil {
		return "Player " + g.Winner.Name + " wins!"
	}

	if g.Player1 == nil && g.Player2 == nil {
		return "Waiting for players"
	} else if g.Player2 == nil {
		return "Waiting for player 2"
	}

	return "Playing " + g.Player1.Name + " vs " + g.Player2.Name
}

func (g *Game) PlayStatus() string {
	if g.Winner != nil {
		return "Game over! " + g.Winner.Name + " wins!"
	}

	if g.Player1 == nil {
		return "Waiting for players"
	} else if g.Player2 == nil {
		return "Waiting for player 2"
	}

	displayName := "Player 1"
	if g.currentPlayer == g.Player2 {
		displayName = "Player 2"
	}
	return "Current player: " + displayName
}

// Returns true if the client that joined is a player
func (g *Game) Join(clientId ParticipantId, name string) bool {
	if g.Player1 != nil && g.Player1.Id == clientId || g.Player2 != nil && g.Player2.Id == clientId {
		return false
	}

	if g.Player1 == nil {
		g.Player1 = g.addParticipant(clientId, name, true)
		return true
	}

	if g.Player2 == nil {
		g.Player2 = g.addParticipant(clientId, name, true)
		g.currentPlayer = g.Player1
		return true
	}

	if p, exists := g.Participants.Get(clientId); exists {
		p.Connected = true
		return false
	}

	g.addParticipant(clientId, name, false)
	return false
}

func (g *Game) addParticipant(id ParticipantId, name string, isPlayer bool) *Participant {
	participant := &Participant{Id: id, Name: name, Player: isPlayer, Connected: true}
	g.Participants.Set(participant.Id, participant)
	return participant
}

// func (g *Game) PlayMove(player int, index int, c chan<- GameId) error {
func (g *Game) PlayMove(player int, index int) error {
	if g.GameOver() {
		return errors.New("The game has already ended")
	}
	if !g.Started() {
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

	g.History = append(g.History, g.Board)
	g.Board.setCell(index, player)

	// if c != nil {
	// 	defer func() {
	// 		c <- g.Id
	// 	}()
	// }

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

// func (g *Game) CurrentPlayer() string {
// 	return g.currentPlayer
// }

func (g *Game) CurrentPlayer() Participant {
	return *g.currentPlayer
}

func (g *Game) GameOver() bool {
	return g.Winner != nil || g.BoardFull()
}

func (g *Game) Started() bool {
	return g.currentPlayer != nil
}

func (g *Game) Player1Name() string {
	if g.Player1 == nil {
		return ""
	}
	return g.Player1.Name
}

func (g *Game) Player2Name() string {
	if g.Player2 == nil {
		return ""
	}
	return g.Player2.Name
}

// Returns the last move played in format (player, cell)
func (g *Game) LastMove() (int, int) {

	if len(g.History) == 0 {
		return -1, 1
	}

	// bin := func(i int) string {
	// 	return fmt.Sprintf("%018b", i)
	// }

	lastBoard := g.History[len(g.History)-1].value
	diff := g.Board.value ^ lastBoard

	if diff&(diff-1) != 0 {
		panic("More than 2 bits changed between boards")
	}

	for i := 0; i < 2*9; i += 2 {
		shifted := (diff >> i)
		cellValue := shifted & 0b11
		if cellValue != 0 {
			// Found two adjacent bits differing
			cellIndex := i / 2
			player := cellValue
			return player, cellIndex
		}
	}

	return -1, -1
}

func UNUSED(x ...interface{}) {}
