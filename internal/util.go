package server

import (
	"bytes"
	"fmt"
	"io"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

func render(c echo.Context, component templ.Component) error {
	return component.Render(c.Request().Context(), c.Response())
}

func renderToString(c echo.Context, component templ.Component) (string, error) {
	var b bytes.Buffer
	wr := &SingleLineWriter{Writer: &b}
	err := component.Render(c.Request().Context(), wr)

	if err != nil {
		return "", err
	}

	return b.String(), nil
}

// func renderTemplate(name string, data interface{}, c echo.Context) (string, error) {
// 	var templateBuf bytes.Buffer
// 	err := c.Echo().Renderer.Render(, name, data, c)
// 	if err != nil {
// 		return "", err
// 	}
// 	return templateBuf.String(), nil
// }

func UNUSED(x ...interface{}) {}

type SingleLineWriter struct {
	Writer io.Writer
	buffer bytes.Buffer
}

func (t *SingleLineWriter) Write(p []byte) (int, error) {
	written := 0
	for _, b := range p {
		if b != '\r' && b != '\n' {
			err := t.buffer.WriteByte(b)
			if err != nil {
				return written, err
			}
			written++
		}
	}

	n, err := t.buffer.WriteTo(t.Writer)
	if err != nil {
		return int(n), err
	}

	t.buffer.Reset()
	return int(n), nil
}

func sendSse(eventName string, msg string, c echo.Context) {
	w := c.Response().Writer
	fmt.Fprintf(w, "event: %s\n", eventName)
	fmt.Fprintf(w, "data: %s\n\n", msg)
	c.Response().Flush()
}