package server

import (
	"bytes"
	"errors"
	"fmt"
	"jay/tictactoe/internal/events"
	tictactoe "jay/tictactoe/pkg"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (this *Server) GameDisplayHandler(c echo.Context) error {
	game, err := getGame(c)
	if err != nil {
		return err
	}
	clientId, err := this.GetClientId(c)
	if err != nil {
		clientId, err = setClientCookie(c)
		if err != nil {
			return errors.New("Could not set client cookie")
		}
	}

	page := GamePage{
		Game:     game,
		ClientId: clientId,
	}
	return c.Render(200, "play", page)
}

func (this *Server) GameStatusHandler(c echo.Context) error {
	idStr := c.FormValue("id")
	gameId, _ := strconv.Atoi(idStr)
	game := tictactoe.Games[gameId-1]
	return c.Render(http.StatusOK, "game-status", game)
}

func (this *Server) LiveGameListHandler(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
	c.Response().Flush()

	indexListener := make(chan *GameStatusEvent)
	this.mu.Lock()
	this.IndexListeners[indexListener] = struct{}{}
	this.mu.Unlock()

	cleanup := func() {
		this.mu.Lock()
		delete(this.IndexListeners, indexListener)
		this.mu.Unlock()
		close(indexListener)
	}

	processEvent := func(gameId tictactoe.GameId) bool {
		if gameId < 1 || int(gameId) > len(tictactoe.Games) {
			return true
		}
		game := tictactoe.Games[gameId-1]

		if gameId != game.Id {
			return true
		}
		w := c.Response().Writer
		fmt.Fprint(w, "event: game_update\n")
		var templateBuf bytes.Buffer
		// err := c.Echo().Renderer.Render(&tictactoe.TemplateWriter{Writer: &templateBuf}, "game-card-oob", game, c)
		err := c.Echo().Renderer.Render(&tictactoe.TemplateWriter{Writer: &templateBuf}, "game-list", newGameList(), c)
		if err != nil {
			log.Fatal(err)
			return true
		}
		w.Write([]byte("data: " + templateBuf.String() + "\n\n"))
		c.Response().Flush()
		log.Println("Sent game update event")
		return true
	}

listenerLoop:
	for {
		select {
		case <-c.Request().Context().Done():
			// log.Printf("Client %s disconnected", clientId)
			cleanup()
			return nil
		case event := <-indexListener:
			if !processEvent(event.gameId) {
				break listenerLoop
			}
		}
	}

	return nil
}

func (this *Server) GameHandler(c echo.Context) error {
	sessionId, err := uuid.NewV7()
	if err != nil {
		return err
	}
	sessionIdStr := sessionId.String()
	game, err := getGame(c)
	if err != nil {
		return err
	}
	clientId, _ := this.GetClientId(c)
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
	c.Response().Flush()

	gameListener := make(chan *GamePlayEvent)
	this.mu.Lock()
	playerJoined := game.Join(clientId, string(clientId))
	eventType := events.SpectatorJoined
	if playerJoined {
		this.GameStatus <- &GameStatusEvent{gameId: game.Id, info: "Player joined"}
		eventType = events.PlayerJoined
	}
	this.GamePlay <- &GamePlayEvent{
		gameId:    game.Id,
		info:      fmt.Sprintf("Client %s joined game (%s)", clientId, sessionIdStr),
		eventType: eventType,
	}
	clientListeners, exists := this.ActiveGameListeners[clientId]
	if !exists {
		clientListeners = make(map[chan<- *GamePlayEvent]struct{})
		this.ActiveGameListeners[clientId] = clientListeners
	}
	clientListeners[gameListener] = struct{}{}
	this.mu.Unlock()

	// Send full page content in case client gets disconnected without refreshing page
	template, _ := renderTemplate("play-partial", GamePage{Game: game, ClientId: clientId}, c)
	sendSse("first-join", template, c)

	cleanup := func() {
		this.mu.Lock()
		// Close listener and then mark player as disconnected if the number of listeners is 0
		delete(clientListeners, gameListener)
		p, exists := game.Participants.Get(clientId)
		if exists && len(clientListeners) == 0 {
			p.Connected = false
		}
		this.mu.Unlock()
		close(gameListener)
		eventType := events.SpectatorLeft
		if p.Player {
			eventType = events.PlayerLeft
		}
		this.GamePlay <- &GamePlayEvent{
			gameId:    game.Id,
			info:      fmt.Sprintf("Client %s disconnected (%s)", clientId, sessionIdStr),
			eventType: eventType,
		}
	}

listenerLoop:
	for {
		select {
		case <-c.Request().Context().Done():
			// log.Printf("Client %s disconnected", clientId)
			cleanup()
			return nil
		case event := <-gameListener:
			if !processGameEvent(c, event, game, clientId) {
				break listenerLoop
			}
		}
	}

	return nil
}

func (this *Server) NewGameHandler(c echo.Context) error {
	this.mu.Lock()
	game := tictactoe.NewGame()
	this.mu.Unlock()
	// log.Println("New game created. Total games:", len(tictactoe.Games))
	this.GameStatus <- &GameStatusEvent{gameId: game.Id, info: "New game created"}
	return c.Render(http.StatusOK, "game-list", newGameList())
	// return c.Render(http.StatusOK, "game-card", game)
}

func (this *Server) BoardHandler(c echo.Context) error {
	idStr := c.FormValue("id")
	gameId, _ := strconv.Atoi(idStr)
	game := tictactoe.Games[gameId-1]
	return c.Render(http.StatusOK, "board", game)
}

func (this *Server) PlayerMoveHandler(c echo.Context) error {
	game, err := getGame(c)
	if err != nil {
		// return c.String(http.StatusInternalServerError, err.Error())
		return err
	}

	if !game.Started() {
		return c.String(http.StatusBadRequest, "Game has not started yet")
	}

	cellIdxStr := c.FormValue("i")
	cellIdx, _ := strconv.Atoi(cellIdxStr)
	clientId, _ := this.GetClientId(c)
	isPlayer1 := game.Player1.Id == clientId
	isPlayer2 := game.Player2.Id == clientId
	playerValue := 0b01
	if !isPlayer1 && !isPlayer2 {
		return c.String(http.StatusForbidden, "You are not a player in this game")
	}
	if !isPlayer1 {
		playerValue = 0b10
	}

	// err = game.PlayMove(playerValue, cellIdx, gamePlay)
	err = game.PlayMove(playerValue, cellIdx)
	// fmt.Println(game.Board.String())
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	} else {
		this.GamePlay <- &GamePlayEvent{
			gameId:    game.Id,
			info:      fmt.Sprintf("Player %d played at cell %d", playerValue, cellIdx),
			eventType: events.MovePlayed,
		}
	}
	// cell := game.GetCell(cellIdx)
	// return c.Render(http.StatusOK, "cell", cell)
	return c.NoContent(http.StatusOK)
}

func (this *Server) IndexHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "index", newGameList())
}

func (this *Server) GameListHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "game-list", newGameList())
}

func renderTemplate(name string, data interface{}, c echo.Context) (string, error) {
	var templateBuf bytes.Buffer
	err := c.Echo().Renderer.Render(&tictactoe.TemplateWriter{Writer: &templateBuf}, name, data, c)
	if err != nil {
		return "", err
	}
	return templateBuf.String(), nil
}

func processGameEvent(c echo.Context, event *GamePlayEvent, game *tictactoe.Game, clientId tictactoe.ParticipantId) bool {

	switch event.eventType {
	case events.Invalid:
		// _, _ = renderTemplate("client-list", GamePage{Game: game, ClientId: clientId}, c)
		log.Println("Invalid event", event)
	case events.SpectatorJoined, events.SpectatorLeft, events.PlayerJoined, events.PlayerLeft:
		t, _ := renderTemplate("client-list", GamePage{Game: game, ClientId: clientId}, c)
		sendSse("clients", t, c)
	case events.MovePlayed:
		_, idx := game.LastMove()
		t, err := renderTemplate("cell", game.GetCell(idx), c)
		if err != nil {
			panic(err)
		}

		time.Sleep(1 * time.Second)
		sendSse(fmt.Sprintf("cell_%d", idx), t, c)

	default:
		log.Println("Unhandled event", event)
	}

	return true
}

func sendSse(eventName string, msg string, c echo.Context) {
	w := c.Response().Writer
	fmt.Fprintf(w, "event: %s\n", eventName)
	fmt.Fprintf(w, "data: %s\n\n", msg)
	c.Response().Flush()
}
