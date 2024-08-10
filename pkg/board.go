package tictactoe

import (
	"errors"
)

type Cell struct {
	Symbol string
	Index  uint
	GameId int
}

type Board struct {
	value int
}

func NewBoard() *Board {
	return &Board{
		value: 0,
	}
}

func (b *Board) SetCell(index int, player uint8) error {
	if player > 0b10 {
		return errors.New("invalid player")
	}
	val := player << (index * 2)
	b.value |= int(val)
	return nil
}

func (b *Board) GetCell(index int) uint8 {
	return uint8((b.value >> (index * 2)) & 0b11)
}

func (b *Board) Symbol(index uint) string {
	switch b.GetCell(int(index)) {
	case 0b00:
		return ""
	case 0b01:
		return "X"
	case 0b10:
		return "O"
	}
	return "?"
}
