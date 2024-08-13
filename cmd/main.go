package main

import (
	server "jay/tictactoe/internal"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	e := echo.New()
	if true {
		e.Use(middleware.Logger())
	}
	e.Static("/images", "images")
	e.Static("/css", "css")

	server := server.NewServer()
	go server.ListenForGameplayEvents()
	go server.ListenForGameStatusEvents()
	e.Use(server.ClientIdMiddleware)
	e.GET("/", server.IndexHandler)
	e.GET("/games/:id", server.GameDisplayHandler)
	e.GET("/games/:id/history/:offset", server.GameHistoryHandler)
	e.GET("/games/:id/board", server.GameBoardHandler)
	e.GET("/gamelist", server.GameListHandler)
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
