package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	tictactoe "jay/tictactoe/pkg"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const COOKIENAME = "tictactoe"

var funcs = template.FuncMap{
	"Iterate": func(count uint) []uint {
		var i uint
		var Items []uint
		for i = 0; i < count; i++ {
			Items = append(Items, i)
		}
		return Items
	},
	"Loop": func(from, to int) <-chan int {
		ch := make(chan int)
		go func() {
			for i := from; i < to; i++ {
				ch <- i
			}
			close(ch)
		}()
		return ch
	},
	"Cells": func(game *tictactoe.Game) <-chan *tictactoe.Cell {
		ch := make(chan *tictactoe.Cell)
		go func() {
			for i := 0; i < 9; i++ {
				cell := &tictactoe.Cell{
					Symbol: game.Board.Symbol(uint(i)),
					Index:  uint(i),
					GameId: game.Id,
				}
				ch <- cell
			}
			close(ch)
		}()
		return ch
	},
}

type Templates struct {
	templates *template.Template
}

func (t *Templates) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func newTemplate() *Templates {
	return &Templates{
		templates: template.Must(template.New("all_templates").Funcs(funcs).ParseGlob("views/*.html")),
	}
}

type GameList struct {
	Games []*tictactoe.Game
}

type GamePage struct {
	DisplayName string
	Game        *tictactoe.Game
	ClientId    string
}

func newGameList() GameList {
	return GameList{
		Games: tictactoe.Games,
	}
}

func setClientCookie(c echo.Context) (string, error) {
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
	return cookie.Value, nil
}

func getClientId(c echo.Context) (string, error) {
	cookie, err := c.Cookie(COOKIENAME)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func removeElement(slice []string, element string) []string {
	for i, v := range slice {
		if v == element {
			// Remove the element from the slice
			return append(slice[:i], slice[i+1:]...)
		}
	}
	// If the element is not found, return the original slice
	return slice
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

func main() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Renderer = newTemplate()
	e.Static("/images", "images")
	e.Static("/css", "css")

	gamePlay := make(chan tictactoe.GameId, 5)
	gameStatus := make(chan tictactoe.GameId, 5)
	activeGameListeners := make(map[chan tictactoe.GameId]struct{})
	indexListeners := make(map[chan tictactoe.GameId]struct{})
	gameParticipants := make(map[tictactoe.GameId][]string)
	var mu sync.Mutex

	go func() {
		for gameId := range gamePlay {
			mu.Lock()
			for listener := range activeGameListeners {
				listener <- gameId
			}
			mu.Unlock()
		}
	}()

	go func() {
		for gameId := range gameStatus {
			mu.Lock()
			for listener := range indexListeners {
				listener <- gameId
			}
			mu.Unlock()
		}
	}()

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
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
	})

	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "index", newGameList())
	})

	e.GET("/games/:id", func(c echo.Context) error {
		game, err := getGame(c)
		if err != nil {
			return err
		}
		client, err := getClientId(c)
		if err != nil {
			client, err = setClientCookie(c)
			if err != nil {
				return errors.New("Could not set client cookie")
			}
		}
		game.Join(client)
		gameStatus <- game.Id
		gamePlay <- game.Id
		display := "Player 1"
		if game.Player2 == client {
			display = "Player 2"
		}
		page := GamePage{
			DisplayName: display,
			Game:        game,
			ClientId:    client,
		}
		return c.Render(200, "play", page)
	})

	e.GET("/gamelist", func(c echo.Context) error {
		return c.Render(http.StatusOK, "game-list", newGameList())
	})

	e.GET("/gamestatus", func(c echo.Context) error {
		idStr := c.FormValue("id")
		gameId, _ := strconv.Atoi(idStr)
		game := tictactoe.Games[gameId-1]
		return c.Render(http.StatusOK, "game-status", game)
	})

	e.GET("/gameboard", func(c echo.Context) error {
		idStr := c.FormValue("id")
		gameId, _ := strconv.Atoi(idStr)
		game := tictactoe.Games[gameId-1]
		return c.Render(http.StatusOK, "board", game)
	})

	e.GET("/gamelivelist", func(c echo.Context) error {
		// clientId, _ := getClientId(c)
		c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
		c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
		c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
		c.Response().Flush()

		indexListener := make(chan tictactoe.GameId)
		mu.Lock()
		indexListeners[indexListener] = struct{}{}
		mu.Unlock()

		// log.Printf("Client %s connected", clientId)

		cleanup := func() {
			mu.Lock()
			delete(indexListeners, indexListener)
			mu.Unlock()
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
			case gameId := <-indexListener:
				if !processEvent(gameId) {
					break listenerLoop
				}
			}
		}

		return nil
	})

	e.GET("/liveboard", func(c echo.Context) error {
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

		gameListener := make(chan tictactoe.GameId)
		mu.Lock()
		activeGameListeners[gameListener] = struct{}{}
		gameParticipants[game.Id] = append(gameParticipants[game.Id], clientId)
		mu.Unlock()

		log.Printf("Client %s connected to game %d", clientId, game.Id)

		cleanup := func() {
			mu.Lock()
			delete(activeGameListeners, gameListener)
			participants, exists := gameParticipants[game.Id]
			if exists {
				gameParticipants[game.Id] = removeElement(participants, clientId)
				game.Spectators[clientId] = false
				// Notify of game update
				gamePlay <- game.Id
			} else {
				log.Println("Could not retrieve participants for game", game.Id)
			}
			mu.Unlock()
			close(gameListener)
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

	listenerLoop:
		for {
			select {
			case <-c.Request().Context().Done():
				log.Printf("Client %s disconnected", clientId)
				cleanup()
				return nil
			case gameId := <-gameListener:
				if !processEvent(gameId) {
					break listenerLoop
				}
			}
		}

		return nil
	})

	e.POST("/newgame", func(c echo.Context) error {
		mu.Lock()
		game := tictactoe.NewGame()
		mu.Unlock()
		// log.Println("New game created. Total games:", len(tictactoe.Games))
		gameStatus <- game.Id
		return c.Render(http.StatusOK, "game-list", newGameList())
		// return c.Render(http.StatusOK, "game-card", game)
	})

	e.POST("/move", func(c echo.Context) error {
		game, err := getGame(c)
		if err != nil {
			// return c.String(http.StatusInternalServerError, err.Error())
			return err
		}
		// idStr := c.FormValue("id")
		cellIdxStr := c.FormValue("i")
		// gameId, _ := strconv.Atoi(idStr)
		// gameId--
		cellIdx, _ := strconv.Atoi(cellIdxStr)
		// game := tictactoe.Games[gameId]
		clientId, _ := getClientId(c)
		isPlayer1 := game.Player1 == clientId
		isPlayer2 := game.Player2 == clientId
		playerValue := 0b01
		if !isPlayer1 && !isPlayer2 {
			return c.String(http.StatusForbidden, "You are not a player in this game")
		}
		if !isPlayer1 {
			playerValue = 0b10
		}

		err = game.PlayMove(playerValue, cellIdx, gamePlay)
		fmt.Println(game.Board.String())
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		cell := game.GetCell(cellIdx)
		return c.Render(http.StatusOK, "cell", cell)
	})

	e.Logger.Fatal(e.Start(":42069"))
}
