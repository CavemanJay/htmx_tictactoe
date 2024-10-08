package server

import (
	"errors"
	"jay/tictactoe/model"
	tictactoe "jay/tictactoe/pkg"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

const COOKIENAME = "tictactoe"
const DEBUG = true

type Server struct {
	Games          map[tictactoe.GameId]*model.ServerGame
	IndexListeners map[chan<- *model.GameStatusEvent]struct{}
	GamePlay       chan *model.GamePlayEvent
	GameStatus     chan *model.GameStatusEvent
	mu             sync.Mutex
	gameCount      atomic.Uint32
}

func (this *Server) newServerGame() *model.ServerGame {
	game := tictactoe.NewGame(
		tictactoe.GameId(this.gameCount.Add(1)),
	)
	return &model.ServerGame{
		Game:      game,
		Listeners: make(map[tictactoe.ParticipantId]map[chan<- *model.GamePlayEvent]struct{}),
	}
}

func NewServer() *Server {

	s := &Server{
		Games:          make(map[tictactoe.GameId]*model.ServerGame),
		IndexListeners: make(map[chan<- *model.GameStatusEvent]struct{}),
		GamePlay:       make(chan *model.GamePlayEvent, 5),
		GameStatus:     make(chan *model.GameStatusEvent, 5),
	}

	player1 := &tictactoe.Participant{Id: "t1", Name: "Testing 1", Player: true}
	player2 := &tictactoe.Participant{Id: "t2", Name: "Testing 2", Player: true}
	spectator1 := &tictactoe.Participant{Id: "t3", Name: "Testing 3"}
	g := tictactoe.Game{
		Id:            0,
		Board:         *tictactoe.NewBoardWithValue(0b010101),
		Player1:       player1,
		Player2:       player2,
		Winner:        player1,
		CurrentPlayer: player1,
		History: []tictactoe.Board{
			*tictactoe.NewBoardWithValue(0b01),
			*tictactoe.NewBoardWithValue(0b0101),
		},
		Participants: orderedmap.New[tictactoe.ParticipantId, *tictactoe.Participant](
			orderedmap.WithInitialData(orderedmap.Pair[tictactoe.ParticipantId, *tictactoe.Participant]{
				Key:   spectator1.Id,
				Value: spectator1,
			})),
	}

	sg := &model.ServerGame{
		Game:      &g,
		Listeners: make(map[tictactoe.ParticipantId]map[chan<- *model.GamePlayEvent]struct{}),
	}

	s.Games[sg.Id] = sg
	sg = s.newServerGame()
	s.Games[sg.Id] = sg

	return s
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
		game := this.Games[event.GameId]
		for _, listeners := range game.Listeners {
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

func (this *Server) getGame(c echo.Context) (*model.ServerGame, error) {
	idStr := c.Param("id")
	idQueryStr := c.QueryParam("id")
	if idStr == "" {
		idStr = idQueryStr
	}
	gameId, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, err
	}
	if game, exists := this.Games[tictactoe.GameId(gameId)]; exists {
		return game, nil
	}

	return nil, errors.New("Game not found")
}
