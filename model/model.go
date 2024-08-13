package model

import (
	"jay/tictactoe/internal/events"
	tictactoe "jay/tictactoe/pkg"
)

type ServerGame struct {
	*tictactoe.Game
	Listeners map[tictactoe.ParticipantId]map[chan<- *GamePlayEvent]struct{}
}

type GamePlayEvent struct {
	GameId    tictactoe.GameId
	Info      string
	EventType events.GamePlayEventType
}

type GameStatusEvent struct {
	GameId tictactoe.GameId
	Info   string
}

type GamePage struct {
	Game     *tictactoe.Game
	ClientId tictactoe.ParticipantId
}

type GameHistoryControls struct {
	Id            tictactoe.GameId
	BackOffset    int
	ForwardOffset int
	Offset        int
	Oob           bool
	CanGoForward  bool
	CanGoBack     bool
	AtCurrent     bool
}
