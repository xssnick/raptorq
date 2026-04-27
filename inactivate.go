package raptorq

import "github.com/xssnick/raptorq/internal/discmath"

type inactivateDecoder struct {
	cols         uint32
	rows         uint32
	wasRow       []bool
	wasCol       []bool
	colCnt       []uint32
	rowCnt       []uint32
	rowXor       []uint32
	rowCntOffset []uint32
	sortedRows   []uint32
	rowPos       []uint32

	rowStarts []uint32
	rowCols   []uint32
	colStarts []uint32
	colRows   []uint32

	pRows        []uint32
	pCols        []uint32
	inactiveCols []uint32
}

func inactivateDecode(arena *matrixArena, l *discmath.MatrixGF256, pi uint32, entries upperMatrixEntries) (side uint32, pRows, pCols []uint32) {
	if !entries.valid() {
		panic("raptorq: upper matrix entries overflow")
	}

	cols := l.ColsNum() - pi
	rows := l.RowsNum()

	dec := inactivateDecoder{
		cols:         cols,
		rows:         rows,
		wasRow:       arena.newBool(int(rows)),
		wasCol:       arena.newBool(int(cols)),
		colCnt:       arena.newU32(int(cols)),
		rowCnt:       arena.newU32(int(rows)),
		rowXor:       arena.newU32(int(rows)),
		pRows:        arena.newU32(int(rows + pi))[:0],
		pCols:        arena.newU32(int(cols + pi))[:0],
		inactiveCols: arena.newU32(int(cols))[:0],
	}

	dec.indexFromEntries(arena, entries)

	dec.sort(arena)
	dec.loop(arena)

	for row := uint32(0); row < dec.rows; row++ {
		if !dec.wasRow[row] {
			dec.pRows = append(dec.pRows, row)
		}
	}

	side = uint32(len(dec.pCols))
	for i, j := 0, len(dec.inactiveCols)-1; i < j; i, j = i+1, j-1 { // reverse array
		dec.inactiveCols[i], dec.inactiveCols[j] = dec.inactiveCols[j], dec.inactiveCols[i]
	}

	for _, col := range dec.inactiveCols {
		dec.pCols = append(dec.pCols, col)
	}

	for i := uint32(0); i < pi; i++ {
		dec.pCols = append(dec.pCols, dec.cols+i)
	}

	return side, dec.pRows, dec.pCols
}

func (dec *inactivateDecoder) indexFromEntries(arena *matrixArena, entries upperMatrixEntries) {
	nonZero := uint32(0)
	for i := uint32(0); i < entries.n; i++ {
		row := entries.rows[i]
		col := entries.cols[i]
		if row >= dec.rows || col >= dec.cols {
			continue
		}

		dec.colCnt[col]++
		dec.rowCnt[row]++
		dec.rowXor[row] ^= col
		nonZero++
	}

	dec.rowStarts = arena.newU32(int(dec.rows + 1))
	dec.colStarts = arena.newU32(int(dec.cols + 1))

	offset := uint32(0)
	for row := uint32(0); row < dec.rows; row++ {
		dec.rowStarts[row] = offset
		offset += dec.rowCnt[row]
	}
	dec.rowStarts[dec.rows] = offset

	offset = 0
	for col := uint32(0); col < dec.cols; col++ {
		dec.colStarts[col] = offset
		offset += dec.colCnt[col]
	}
	dec.colStarts[dec.cols] = offset

	dec.rowCols = arena.newU32(int(nonZero))
	dec.colRows = arena.newU32(int(nonZero))

	rowCursor := arena.newU32(int(dec.rows))
	colCursor := arena.newU32(int(dec.cols))
	copy(rowCursor, dec.rowStarts[:dec.rows])
	copy(colCursor, dec.colStarts[:dec.cols])

	for i := uint32(0); i < entries.n; i++ {
		row := entries.rows[i]
		col := entries.cols[i]
		if row >= dec.rows || col >= dec.cols {
			continue
		}

		rowPos := rowCursor[row]
		dec.rowCols[rowPos] = col
		rowCursor[row]++

		colPos := colCursor[col]
		dec.colRows[colPos] = row
		colCursor[col]++
	}
}

func (dec *inactivateDecoder) rowColumns(row uint32) []uint32 {
	return dec.rowCols[dec.rowStarts[row]:dec.rowStarts[row+1]]
}

func (dec *inactivateDecoder) columnRows(col uint32) []uint32 {
	return dec.colRows[dec.colStarts[col]:dec.colStarts[col+1]]
}

func (dec *inactivateDecoder) sort(arena *matrixArena) {
	offset := arena.newU32(int(dec.cols + 2))
	for i := uint32(0); i < dec.rows; i++ {
		offset[dec.rowCnt[i]+1]++
	}
	for i := uint32(1); i <= dec.cols+1; i++ {
		offset[i] += offset[i-1]
	}
	dec.rowCntOffset = arena.newU32(int(dec.rows))
	copy(dec.rowCntOffset, offset)

	dec.sortedRows = arena.newU32(int(dec.rows))
	dec.rowPos = arena.newU32(int(dec.rows))
	for i := uint32(0); i < dec.rows; i++ {
		pos := offset[dec.rowCnt[i]]
		offset[dec.rowCnt[i]]++

		dec.sortedRows[pos] = i
		dec.rowPos[i] = pos
	}
}

func (dec *inactivateDecoder) loop(arena *matrixArena) {
	// loop
	for dec.rowCntOffset[1] != dec.rows {
		row := dec.sortedRows[dec.rowCntOffset[1]]
		col := dec.chooseCol(row)

		cnt := dec.rowCnt[row]
		dec.pCols = append(dec.pCols, col)
		dec.pRows = append(dec.pRows, row)

		if cnt == 1 {
			dec.inactivate(col)
		} else {
			for _, x := range dec.rowColumns(row) {
				if dec.wasCol[x] {
					continue
				}
				if x != col {
					dec.inactiveCols = append(dec.inactiveCols, x)
				}
				dec.inactivate(x)
			}
		}
		dec.wasRow[row] = true
	}
}

func (dec *inactivateDecoder) chooseCol(row uint32) uint32 {
	cnt := dec.rowCnt[row]
	if cnt == 1 {
		return dec.rowXor[row]
	}

	bestCol := uint32(0xFFFFFFFF)
	for _, col := range dec.rowColumns(row) {
		if dec.wasCol[col] {
			continue
		}
		if bestCol == 0xFFFFFFFF || dec.colCnt[col] < dec.colCnt[bestCol] {
			bestCol = col
		}
	}
	return bestCol
}

func (dec *inactivateDecoder) inactivate(col uint32) {
	dec.wasCol[col] = true
	for _, row := range dec.columnRows(col) {
		if dec.wasRow[row] {
			continue
		}

		pos := dec.rowPos[row]
		cnt := dec.rowCnt[row]
		offset := dec.rowCntOffset[cnt]
		dec.sortedRows[pos], dec.sortedRows[offset] = dec.sortedRows[offset], dec.sortedRows[pos]

		dec.rowPos[dec.sortedRows[pos]] = pos
		dec.rowPos[dec.sortedRows[offset]] = offset
		dec.rowCntOffset[cnt]++
		dec.rowCnt[row]--
		dec.rowXor[row] ^= col
	}
}
