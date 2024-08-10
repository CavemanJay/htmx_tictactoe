package main

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	tictactoe "jay/tictactoe/pkg"
	"net/http"
	"strconv"

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

type IndexPage struct {
	Games []*tictactoe.Game
}

type GamePage struct {
	DisplayName string
	Game        *tictactoe.Game
}

func newIndexPage() IndexPage {
	return IndexPage{
		Games: tictactoe.Games,
	}
}

func setClientCookie(c echo.Context) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	cookie := new(http.Cookie)
	cookie.Name = COOKIENAME
	cookie.Value = id.String()
	cookie.Path = "/"
	c.SetCookie(cookie)
	return nil
}

func getClientId(c echo.Context) (string, error) {
	cookie, err := c.Cookie(COOKIENAME)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Renderer = newTemplate()
	e.Static("/images", "images")
	e.Static("/css", "css")

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			_, err := c.Cookie(COOKIENAME)
			if err != nil {
				switch {
				case errors.Is(err, http.ErrNoCookie):
					err = setClientCookie(c)
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
		return c.Render(http.StatusOK, "index", newIndexPage())
	})

	e.GET("/games/:id", func(c echo.Context) error {
		idStr := c.Param("id")
		id, _ := strconv.Atoi(idStr)
		id--
		if id < 0 || id >= len(tictactoe.Games) {
			return c.String(http.StatusNotFound, "Game not found")
		}
		game := tictactoe.Games[id]
		display := "Player 1"
		client, _ := getClientId(c)
		game.Join(client)
		if game.Player2 == client {
			display = "Player 2"
		}
		page := GamePage{
			DisplayName: display,
			Game:        game,
		}
		return c.Render(200, "play", page)
	})

	e.GET("/gamelist", func(c echo.Context) error {
		return c.Render(http.StatusOK, "game-list", newIndexPage())
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

	e.POST("/newgame", func(c echo.Context) error {
		tictactoe.NewGame()
		return c.Render(http.StatusOK, "game-list", newIndexPage())
	})

	e.POST("/move", func(c echo.Context) error {
		idStr := c.FormValue("id")
		cellIdxStr := c.FormValue("i")
		gameId, _ := strconv.Atoi(idStr)
		gameId--
		cellIdx, _ := strconv.Atoi(cellIdxStr)
		game := tictactoe.Games[gameId]
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
		err := game.PlayMove(playerValue, cellIdx)
		fmt.Println(game.Board.String())
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		cell := game.GetCell(cellIdx)
		return c.Render(http.StatusOK, "cell", cell)
	})

	e.Logger.Fatal(e.Start(":42069"))
}
