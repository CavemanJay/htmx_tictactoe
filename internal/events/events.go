package events

type GamePlayEventType int

const (
	Invalid GamePlayEventType = iota
	PlayerJoined
	PlayerLeft
	SpectatorJoined
	SpectatorLeft
	MovePlayed
	GameOver
)
