package server

import (
	"bytes"
	"errors"
	"fmt"
	"jay/tictactoe/internal/events"
	"jay/tictactoe/model"
	tictactoe "jay/tictactoe/pkg"
	"jay/tictactoe/view"
	"jay/tictactoe/view/shared"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (this *Server) GameHistoryHandler(c echo.Context) error {
	game, err := this.getGame(c)
	if err != nil {
		return err
	}
	offsetStr := c.Param("offset")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		return err
	}
	gameHistoryControls := model.GameHistoryControls{
		Id:            game.Id,
		BackOffset:    offset - 1,
		Offset:        offset,
		ForwardOffset: offset + 1,
		CanGoBack:     offset*-1 < len(game.History),
		CanGoForward:  offset < 0,
		AtCurrent:     offset == 0,
		Oob:           true,
	}
	type ControlsData struct {
		tictactoe.Board
		model.GameHistoryControls
	}
	board := game.Board
	if offset < 0 {
		board = game.History[len(game.History)+offset]
	}
	data := &ControlsData{board, gameHistoryControls}
	err = c.Echo().Renderer.Render(
		c.Response().Writer,
		"board-history",
		data,
		c,
	)
	if err != nil {
		return err
	}

	return c.Render(200, "history-controls", &gameHistoryControls)
}

func (this *Server) GameDisplayHandler(c echo.Context) error {
	game, err := this.getGame(c)
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

	return render(c, view.Game(game.Game, clientId))
}

func (this *Server) LiveGameListHandler(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
	c.Response().Flush()

	indexListener := make(chan *model.GameStatusEvent)
	this.mu.Lock()
	this.IndexListeners[indexListener] = struct{}{}
	this.mu.Unlock()

	cleanup := func() {
		this.mu.Lock()
		delete(this.IndexListeners, indexListener)
		this.mu.Unlock()
		close(indexListener)
	}

	processEvent := func(event *model.GameStatusEvent) bool {
		// game := this.Games[event.gameId].Game

		w := c.Response().Writer
		fmt.Fprint(w, "event: game_update\n")
		s, err := renderToString(c, view.GameList(this.gameList()))
		if err != nil {
			log.Fatal(err)
			return true
		}
		w.Write([]byte("data: " + s + "\n\n"))
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
			if !processEvent(event) {
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
	game, err := this.getGame(c)
	if err != nil {
		return err
	}
	clientId, _ := this.GetClientId(c)
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
	c.Response().Flush()

	gameListener := make(chan *model.GamePlayEvent)
	this.mu.Lock()
	playerJoined := game.Join(clientId, string(clientId))
	eventType := events.SpectatorJoined
	if playerJoined {
		this.GameStatus <- &model.GameStatusEvent{GameId: game.Id, Info: "Player joined"}
		eventType = events.PlayerJoined
	}
	this.GamePlay <- &model.GamePlayEvent{
		GameId:    game.Id,
		Info:      fmt.Sprintf("Client %s joined game (%s)", clientId, sessionIdStr),
		EventType: eventType,
	}
	// clientListeners, exists := this.ActiveGameListeners[clientId]
	clientListeners, exists := this.Games[game.Id].Listeners[clientId]
	if !exists {
		clientListeners = make(map[chan<- *model.GamePlayEvent]struct{})
		this.Games[game.Id].Listeners[clientId] = clientListeners
	}
	clientListeners[gameListener] = struct{}{}
	this.mu.Unlock()

	// Send full page content in case client gets disconnected without refreshing page
	template, err := renderToString(c, view.GamePartial(game.Game, clientId))
	if err != nil {
		return err
	}
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
		this.GamePlay <- &model.GamePlayEvent{
			GameId:    game.Id,
			Info:      fmt.Sprintf("Client %s disconnected (%s)", clientId, sessionIdStr),
			EventType: eventType,
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
	game := this.newServerGame()
	this.Games[game.Id] = game
	this.mu.Unlock()
	// log.Println("New game created. Total games:", len(tictactoe.Games))
	this.GameStatus <- &model.GameStatusEvent{GameId: game.Id, Info: "New game created"}
	return c.Render(http.StatusOK, "game-list", this)
	// return c.Render(http.StatusOK, "game-card", game)
}

func (this *Server) GameBoardHandler(c echo.Context) error {
	game, err := this.getGame(c)
	if err != nil {
		return err
	}
	// c.Request().Header.Get("Hx-Request")
	// return c.Render(http.StatusOK, "board", game)

	return render(c, shared.Board(game.Game))
}

func (this *Server) PlayerMoveHandler(c echo.Context) error {
	game, err := this.getGame(c)
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
		this.GamePlay <- &model.GamePlayEvent{
			GameId:    game.Id,
			Info:      fmt.Sprintf("Player %d played at cell %d", playerValue, cellIdx),
			EventType: events.MovePlayed,
		}
	}
	// cell := game.GetCell(cellIdx)
	// return c.Render(http.StatusOK, "cell", cell)
	return c.NoContent(http.StatusOK)
}

func (this *Server) IndexHandler(c echo.Context) error {
	return render(c, view.Index(this.gameList()))
}

func (this *Server) GameListHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "game-list", this)
}

// func renderTemplate(name string, data interface{}, c echo.Context) (string, error) {
// 	var templateBuf bytes.Buffer
// 	err := c.Echo().Renderer.Render(&tictactoe.TemplateWriter{Writer: &templateBuf}, name, data, c)
// 	if err != nil {
// 		return "", err
// 	}
// 	return templateBuf.String(), nil
// }

func processGameEvent(c echo.Context, event *model.GamePlayEvent, game *model.ServerGame, clientId tictactoe.ParticipantId) bool {
	sendError := func(err error) {
		var b bytes.Buffer
		w := SingleLineWriter{Writer: &b}
		w.Write([]byte(err.Error()))
		sendSse("error", b.String(), c)
	}

	switch event.EventType {
	case events.Invalid:
		// _, _ = renderTemplate("client-list", GamePage{Game: game, ClientId: clientId}, c)
		log.Println("Invalid event", event)
	case events.SpectatorJoined, events.SpectatorLeft, events.PlayerJoined, events.PlayerLeft:
		// t, _ := renderTemplate("client-list", model.GamePage{Game: game.Game, ClientId: clientId}, c)
		t, err := renderToString(c, shared.Clients(game.Game, clientId))
		if err != nil {
			sendError(err)
		} else {
			sendSse("clients", t, c)
		}
	case events.MovePlayed:
		_, idx := game.LastMove()
		// t, err := renderTemplate("cell", game.GetCell(idx), c)
		t, err := renderToString(c, shared.Cell(game.GetCell(idx), game.Id, false))
		if err != nil {
			sendError(err)
		}

		time.Sleep(200 * time.Millisecond)
		sendSse(fmt.Sprintf("cell_%d", idx), t, c)

		if game.GameOver() {
			sendSse("game_over", "", c)
		}

	default:
		log.Println("Unhandled event", event)
	}

	return true
}

func (this *Server) gameList() []*tictactoe.Game {

	var games []*tictactoe.Game
	for _, game := range this.Games {
		games = append(games, game.Game)
	}
	return games
}
