package main

import (
	"html/template"
	"io"
	server "jay/tictactoe/internal"
	tictactoe "jay/tictactoe/pkg"
	"math/rand"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var templateFuncs = template.FuncMap{
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
	"Spectators": func(game *tictactoe.Game) <-chan *tictactoe.Participant {
		ch := make(chan *tictactoe.Participant, game.Participants.Len())
		go func() {
			for pair := game.Participants.Oldest(); pair != nil; pair = pair.Next() {
				if pair.Value.Player {
					continue
				}

				ch <- pair.Value
			}
			close(ch)
		}()
		return ch
	},
	"Games": func(server server.Server) <-chan *tictactoe.Game {
		ch := make(chan *tictactoe.Game, len(server.Games))
		go func() {
			for _, game := range server.Games {
				ch <- game.Game
			}
			close(ch)
		}()
		return ch
	},
	"Rand": func() int {
		rand.Seed(time.Now().UnixNano())
		return rand.Intn(100)
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
		templates: template.Must(template.New("all_templates").Funcs(templateFuncs).ParseGlob("views/*.html")),
	}
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

func main() {
	e := echo.New()
	if true {
		e.Use(middleware.Logger())
	}
	e.Renderer = newTemplate()
	e.Static("/images", "images")
	e.Static("/css", "css")

	server := server.NewServer()
	go server.ListenForGameplayEvents()
	go server.ListenForGameStatusEvents()
	e.Use(server.ClientIdMiddleware)
	e.GET("/", server.IndexHandler)
	e.GET("/games/:id", server.GameDisplayHandler)
	e.GET("/gamelist", server.GameListHandler)
	// e.GET("/gamestatus", server.GameStatusHandler)
	e.GET("/gameboard", server.BoardHandler)
	e.GET("/livegamelist", server.LiveGameListHandler)
	e.GET("/liveboard/:id", server.GameHandler)
	e.GET("/is-this-me", func(c echo.Context) error {
		clientId, _ := server.GetClientId(c)
		query := c.QueryParam("id")
		if query == string(clientId) {
			return c.String(200, "(You)")
		}

		return c.String(200, "")
	})
	e.POST("/newgame", server.NewGameHandler)
	e.POST("/move", server.PlayerMoveHandler)
	e.Logger.Fatal(e.Start(":42069"))
}
