package tictactoe

import (
	"errors"
	"fmt"
)

type Cell struct {
	Symbol string
	Index  uint
	GameId GameId
}

type Board struct {
	value int
}

func NewBoardWithValue(val int) *Board {
	return &Board{
		value: val,
	}
}

func NewBoard() *Board {
	return &Board{
		value: 0,
	}
}

func (b *Board) setCell(index int, player int) error {
	if player > 0b10 {
		return errors.New("invalid player")
	}
	val := int(player) << (index * 2)
	b.value |= int(val)
	return nil
}

func (b *Board) GetCell(index int) int {
	return (b.value >> (index * 2)) & 0b11
}

func (b *Board) Bin() string {
	return fmt.Sprintf("%018b", b.value)
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

func (b *Board) String() string {
	val := ""
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			s := b.Symbol(uint(r*3 + c))
			if s == "" {
				s = "?"
			}
			val += s
		}

		val += "\n"
	}

	val += "\n" + b.Bin()

	return val
}
