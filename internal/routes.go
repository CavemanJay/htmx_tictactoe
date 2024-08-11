package server

import (
	"bytes"
	"errors"
	"fmt"
	tictactoe "jay/tictactoe/pkg"
	"log"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

func (this *Server) GameDisplayHandler(c echo.Context) error {
	game, err := getGame(c)
	if err != nil {
		return err
	}
	clientId, err := getClientId(c)
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
	game, err := getGame(c)
	if err != nil {
		// return c.String(http.StatusInternalServerError, err.Error())
		return err
	}
	clientId, _ := getClientId(c)
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
	c.Response().Flush()

	gameListener := make(chan *GamePlayEvent)
	this.mu.Lock()
	playerJoined := game.Join(clientId, string(clientId))
	if playerJoined {
		this.GameStatus <- &GameStatusEvent{gameId: game.Id, info: "Player joined"}
	}
	this.GamePlay <- &GamePlayEvent{gameId: game.Id, info: fmt.Sprintf("Client %s joined game", clientId)}
	clientListeners, exists := this.ActiveGameListeners[clientId]
	if !exists {
		clientListeners = make(map[chan<- *GamePlayEvent]struct{})
		this.ActiveGameListeners[clientId] = clientListeners
	}
	clientListeners[gameListener] = struct{}{}
	x := len(this.ActiveGameListeners)
	fmt.Println("Active game listeners:", x)
	this.mu.Unlock()

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
		this.GamePlay <- &GamePlayEvent{gameId: game.Id, info: fmt.Sprintf("Client %s disconnected", clientId)}
	}

	processEvent := func(gameId tictactoe.GameId) bool {
		if gameId != game.Id {
			return true
		}
		w := c.Response().Writer
		fmt.Fprintf(w, "event: game_refresh_%d\n", gameId)
		var templateBuf bytes.Buffer
		c.Echo().Renderer.Render(&tictactoe.TemplateWriter{Writer: &templateBuf}, "board", game, c)
		c.Echo().Renderer.Render(&tictactoe.TemplateWriter{Writer: &templateBuf}, "game-status", game, c)
		c.Echo().Renderer.Render(&tictactoe.TemplateWriter{Writer: &templateBuf}, "client-list", GamePage{Game: game, ClientId: clientId}, c)
		if game.GameOver() {
			c.Echo().Renderer.Render(&tictactoe.TemplateWriter{Writer: &templateBuf}, "history-controls", game, c)
		}
		w.Write([]byte("data: " + templateBuf.String() + "\n\n"))
		c.Response().Flush()

		// if game.GameOver() {
		// 	w.Write([]byte("event: game_over\n"))
		// 	w.Write([]byte("data: \n\n"))
		// 	c.Response().Flush()
		// 	cleanup()
		// 	return false
		// }

		return true
	}

	ctx := c.Request().Context()
listenerLoop:
	for {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			// log.Printf("Client %s disconnected", clientId)
			cleanup()
			return nil
		case event := <-gameListener:
			if !processEvent(event.gameId) {
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
	clientId, _ := getClientId(c)
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
		this.GamePlay <- &GamePlayEvent{gameId: game.Id, info: fmt.Sprintf("Player %d played at cell %d", playerValue, cellIdx)}
	}
	cell := game.GetCell(cellIdx)
	return c.Render(http.StatusOK, "cell", cell)
}

func (this *Server) IndexHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "index", newGameList())
}

func (this *Server) GameListHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "game-list", newGameList())
}
