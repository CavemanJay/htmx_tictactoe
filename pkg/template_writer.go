package tictactoe

import (
	"bytes"
	"io"
)

type TemplateWriter struct {
	Writer io.Writer
	buffer         bytes.Buffer
}

func (t *TemplateWriter) Write(p []byte) (int, error) {
	 written  := 0
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
