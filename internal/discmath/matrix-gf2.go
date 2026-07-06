package discmath

import (
	"encoding/binary"
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
	firstElIdx := row * m.rowSize
	OctVecAdd(m.data[firstElIdx:firstElIdx+uint32(len(what))], what)
}

func (m *PlainMatrixGF2) Mul(s *MatrixGF256) *PlainMatrixGF2 {
	mg := NewPlainMatrixGF2(s.RowsNum(), m.ColsNum())
	return m.MulTo(s, mg)
}

// MulTo accumulates into mg, which must be zeroed (both callers pass
// freshly initialized matrices, InitPlainMatrixGF2 already clears)
func (m *PlainMatrixGF2) MulTo(s *MatrixGF256, mg *PlainMatrixGF2) *PlainMatrixGF2 {
	for row := uint32(0); row < s.Rows; row++ {
		for col, val := range s.GetRow(row) {
			if val != 0 {
				mg.RowAdd(row, m.GetRow(uint32(col)))
			}
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

// _BitExpand maps a bit-packed byte to 8 result bytes (0 or 1 each),
// bit i of the input becomes byte i of the little-endian uint64.
var _BitExpand = calcBitExpand()

func calcBitExpand() [256]uint64 {
	var t [256]uint64
	for b := 0; b < 256; b++ {
		var v uint64
		for bit := 0; bit < 8; bit++ {
			if b&(1<<bit) != 0 {
				v |= 1 << (8 * bit)
			}
		}
		t[b] = v
	}
	return t
}

func (m *PlainMatrixGF2) RowToGF256(row uint32, dst []byte) {
	dst = dst[:m.cols]
	rowData := m.GetRow(row)

	full := int(m.cols / elSize)
	for i := 0; i < full; i++ {
		binary.LittleEndian.PutUint64(dst[i*elSize:], _BitExpand[rowData[i]])
	}

	if col := uint32(full * elSize); col < m.cols {
		b := rowData[full]
		for bit := byte(0); col < m.cols; bit++ {
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
