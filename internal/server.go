package server

import (
	"errors"
	"jay/tictactoe/internal/events"
	tictactoe "jay/tictactoe/pkg"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const COOKIENAME = "tictactoe"

type GamePlayEvent struct {
	gameId       tictactoe.GameId
	info         string
	eventType    events.GamePlayEventType
	// SseEventName string
}


type GameStatusEvent struct {
	gameId tictactoe.GameId
	info   string
}

type Server struct {
	ActiveGameListeners map[tictactoe.ParticipantId]map[chan<- *GamePlayEvent]struct{}
	IndexListeners      map[chan<- *GameStatusEvent]struct{}
	GamePlay            chan *GamePlayEvent
	GameStatus          chan *GameStatusEvent
	mu                  sync.Mutex
}

type GameList struct {
	Games []*tictactoe.Game
}

type GamePage struct {
	Game     *tictactoe.Game
	ClientId tictactoe.ParticipantId
}

func newGameList() GameList {
	return GameList{
		Games: tictactoe.Games,
	}
}

func NewServer() *Server {
	return &Server{
		ActiveGameListeners: make(map[tictactoe.ParticipantId]map[chan<- *GamePlayEvent]struct{}),
		IndexListeners:      make(map[chan<- *GameStatusEvent]struct{}),
		GamePlay:            make(chan *GamePlayEvent, 5),
		GameStatus:          make(chan *GameStatusEvent, 5),
	}
}

func (this *Server) ClientIdMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		_, err := c.Cookie(COOKIENAME)
		if err != nil {
			switch {
			case errors.Is(err, http.ErrNoCookie):
				_, err = setClientCookie(c)
				if err != nil {
					return err
				}
			default:
				return err
			}
		}
		return next(c)
	}
}

func (this *Server) ListenForGameplayEvents() {
	for event := range this.GamePlay {
		log.Println("Game play event received:", event)
		this.mu.Lock()
		for _, listeners := range this.ActiveGameListeners {
			for listener := range listeners {
				listener <- event
			}
		}
		this.mu.Unlock()
	}
}

func (this *Server) ListenForGameStatusEvents() {
	for event := range this.GameStatus {
		log.Println("Game status event received:", event)
		this.mu.Lock()
		for listener := range this.IndexListeners {
			listener <- event
		}
		this.mu.Unlock()
	}
}

func setClientCookie(c echo.Context) (tictactoe.ParticipantId, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}

	idStr := id.String()
	idParts := strings.Split(idStr, "-")
	x := idParts[len(idParts)-2] + "-" + idParts[len(idParts)-1]
	cookie := new(http.Cookie)
	cookie.Name = COOKIENAME
	cookie.Value = x
	cookie.Path = "/"
	c.SetCookie(cookie)
	c.Response().Flush()
	return tictactoe.ParticipantId(cookie.Value), nil
}

func (this *Server) GetClientId(c echo.Context) (tictactoe.ParticipantId, error) {
	cookie, err := c.Cookie(COOKIENAME)
	if err != nil {
		return "", err
	}
	return tictactoe.ParticipantId(cookie.Value), nil
}

func getGame(c echo.Context) (*tictactoe.Game, error) {
	idStr := c.Param("id")
	idQueryStr := c.QueryParam("id")
	if idStr == "" {
		idStr = idQueryStr
	}
	gameId, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, err
	}
	gameId--
	if gameId < 0 || gameId >= len(tictactoe.Games) {
		return nil, errors.New("Game not found")
		// return nil, c.String(http.StatusNotFound, "Game not found")
	}
	return tictactoe.Games[gameId], nil
}
