package discmath

import (
	"fmt"
	"strings"
)

// elSize is a size of array's element in bits
const elSize = 8

type PlainMatrixGF2 struct {
	rows, cols uint32
	rowSize    uint32
	data       []byte
}

func NewPlainMatrixGF2(rows, cols uint32) *PlainMatrixGF2 {
	data := make([]byte, PlainMatrixGF2DataSize(rows, cols))
	m := &PlainMatrixGF2{}
	InitPlainMatrixGF2(m, rows, cols, data)
	return m
}

func PlainMatrixGF2DataSize(rows, cols uint32) int {
	rowSize := cols / elSize
	if cols%elSize > 0 {
		rowSize++
	}
	return int(rows * rowSize)
}

func InitPlainMatrixGF2(m *PlainMatrixGF2, rows, cols uint32, data []byte) {
	rowSize := cols / elSize
	if cols%elSize > 0 {
		rowSize++
	}
	data = data[:rows*rowSize]
	clear(data)
	*m = PlainMatrixGF2{
		rows:    rows,
		cols:    cols,
		rowSize: rowSize,
		data:    data,
	}
}

func (m *PlainMatrixGF2) RowsNum() uint32 {
	return m.rows
}

func (m *PlainMatrixGF2) ColsNum() uint32 {
	return m.cols
}

func (m *PlainMatrixGF2) Get(row, col uint32) byte {
	return m.getElement(row, col)
}

func (m *PlainMatrixGF2) Set(row, col uint32) {
	elIdx, colIdx := m.getElementPosition(row, col)
	m.data[elIdx] |= 1 << colIdx
}

func (m *PlainMatrixGF2) Unset(row, col uint32) {
	elIdx, colIdx := m.getElementPosition(row, col)
	m.data[elIdx] &= ^(1 << colIdx)
}

func (m *PlainMatrixGF2) GetRow(row uint32) []byte {
	firstElIdx, _ := m.getElementPosition(row, 0)
	lastElIdx := firstElIdx + (m.cols-1)/elSize + 1

	return m.data[firstElIdx:lastElIdx]
}

func (m *PlainMatrixGF2) RowAdd(row uint32, what []byte) {
	firstElIdx, _ := m.getElementPosition(row, 0)
	for i, whatByte := range what {
		m.data[firstElIdx+uint32(i)] ^= whatByte
	}
}

func (m *PlainMatrixGF2) Mul(s *MatrixGF256) *PlainMatrixGF2 {
	mg := NewPlainMatrixGF2(s.RowsNum(), m.ColsNum())
	return m.MulTo(s, mg)
}

func (m *PlainMatrixGF2) MulTo(s *MatrixGF256, mg *PlainMatrixGF2) *PlainMatrixGF2 {
	clear(mg.data)

	for i, val := range s.Data {
		if val != 0 {
			row := uint32(i) / s.Cols
			col := uint32(i) % s.Cols

			mRow := m.GetRow(col)
			mg.RowAdd(row, mRow)
		}
	}

	return mg
}

func (m *PlainMatrixGF2) ToGF256() *MatrixGF256 {
	mg := NewMatrixGF256(m.RowsNum(), m.ColsNum())

	result := make([]uint8, m.cols)
	for i := uint32(0); i < m.rows; i++ {
		for col := uint32(0); col < m.cols; col++ {
			result[col] = m.getElement(i, col)
		}
		mg.RowSet(i, result)
	}

	return mg
}

func (m *PlainMatrixGF2) String() string {
	var rows []string
	for row := uint32(0); row < m.rows; row++ {
		var cols []string
		for col := uint32(0); col < m.cols; col++ {
			cols = append(cols, fmt.Sprintf("%02x", m.getElement(row, col)))
		}

		rows = append(rows, strings.Join(cols, " "))
	}

	return strings.Join(rows, "\n")
}

func (m *PlainMatrixGF2) RowToGF256(row uint32, dst []byte) {
	dst = dst[:m.cols]
	clear(dst)

	col := uint32(0)
	for _, b := range m.GetRow(row) {
		for bit := byte(0); bit < elSize && col < m.cols; bit++ {
			dst[col] = (b >> bit) & 1
			col++
		}
	}
}

// getElement returns element in matrix by row and col. Possible values: 0 or 1
func (m *PlainMatrixGF2) getElement(row, col uint32) byte {
	elIdx, colIdx := m.getElementPosition(row, col)

	return (m.data[elIdx] & (1 << colIdx)) >> colIdx
}

// getElementPosition returns index of element in array and offset in this element
func (m *PlainMatrixGF2) getElementPosition(row, col uint32) (uint32, byte) {
	return (row * m.rowSize) + col/elSize, byte(col % elSize)
}
