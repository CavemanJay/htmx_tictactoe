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
	"github.com/unrolled/render"
)

const COOKIENAME = "tictactoe"

type Templates struct {
	templates *template.Template
}

type RenderWrapper struct {
	rnd *render.Render
}

func (r *RenderWrapper) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return r.rnd.HTML(w, 0, name, data)
}

// func (t *Templates) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
// 	return t.templates.ExecuteTemplate(w, name, data)
// }

func newTemplate() *Templates {
	funcs := template.FuncMap{
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
	// e.Renderer = newTemplate()
	e.Renderer = &RenderWrapper{rnd: render.New(render.Options{
		Directory: "views",
	})}
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
		return c.Render(http.StatusOK, "index.html", newIndexPage())
	})

	e.GET("/games/:id", func(c echo.Context) error {
		idStr := c.Param("id")
		id, _ := strconv.Atoi(idStr)
		id--
		if id < 0 || id >= len(tictactoe.Games) {
			return c.String(http.StatusNotFound, "Game not found")
		}

		game := tictactoe.Games[id]
		page := GamePage{
			DisplayName: "Player 2",
			Game:        game,
		}
		fmt.Println(page.Game.PlayStatus())
		return c.Render(200, "play.html", page)
	})

	e.GET("/gamelist", func(c echo.Context) error {
		return c.Render(http.StatusOK, "game-list", newIndexPage())
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
		game.Board.SetCell(cellIdx, 2)
		cell := game.GetCell(cellIdx)
		return c.Render(http.StatusOK, "cell", cell)
	})

	e.Logger.Fatal(e.Start(":42069"))
}
