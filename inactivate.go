package raptorq

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

func inactivateDecode(arena *matrixArena, lRows, lCols, pi uint32, entries upperMatrixEntries) (side uint32, pRows, pCols []uint32) {
	if !entries.valid() {
		panic("raptorq: upper matrix entries overflow")
	}

	cols := lCols - pi
	rows := lRows

	dec := inactivateDecoder{
		cols:   cols,
		rows:   rows,
		wasRow: arena.newBool(int(rows)),
		wasCol: arena.newBool(int(cols)),
		colCnt: arena.newU32(int(cols)),
		rowCnt: arena.newU32(int(rows)),
		rowXor: arena.newU32(int(rows)),
		// append targets, written before any read
		pRows:        arena.newU32Dirty(int(rows + pi))[:0],
		pCols:        arena.newU32Dirty(int(cols + pi))[:0],
		inactiveCols: arena.newU32Dirty(int(cols))[:0],
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

	dec.pCols = append(dec.pCols, dec.inactiveCols...)

	n := len(dec.pCols)
	dec.pCols = dec.pCols[:n+int(pi)]
	for i := uint32(0); i < pi; i++ {
		dec.pCols[n+int(i)] = dec.cols + i
	}

	return side, dec.pRows, dec.pCols
}

func (dec *inactivateDecoder) indexFromEntries(arena *matrixArena, entries upperMatrixEntries) {
	eRows := entries.rows[:entries.n]
	eCols := entries.cols[:entries.n]

	rowCnt := dec.rowCnt
	colCnt := dec.colCnt
	rowXor := dec.rowXor[:len(rowCnt)]
	for i, row := range eRows {
		col := eCols[i]
		r, c := int(row), int(col)
		if r >= len(rowCnt) || c >= len(colCnt) {
			continue
		}

		colCnt[c]++
		rowCnt[r]++
		rowXor[r] ^= col
	}

	// starts are fully written by the prefix sums, cols/rows arrays by the
	// cursor scatter below, so all can start dirty
	dec.rowStarts = arena.newU32Dirty(int(dec.rows + 1))
	dec.colStarts = arena.newU32Dirty(int(dec.cols + 1))

	offset := uint32(0)
	for row := uint32(0); row < dec.rows; row++ {
		dec.rowStarts[row] = offset
		offset += rowCnt[row]
	}
	dec.rowStarts[dec.rows] = offset
	nonZero := offset

	offset = 0
	for col := uint32(0); col < dec.cols; col++ {
		dec.colStarts[col] = offset
		offset += colCnt[col]
	}
	dec.colStarts[dec.cols] = offset

	dec.rowCols = arena.newU32Dirty(int(nonZero))
	dec.colRows = arena.newU32Dirty(int(nonZero))

	rowCursor := arena.newU32Dirty(int(dec.rows))
	colCursor := arena.newU32Dirty(int(dec.cols))
	copy(rowCursor, dec.rowStarts[:dec.rows])
	copy(colCursor, dec.colStarts[:dec.cols])

	rowCols := dec.rowCols
	colRows := dec.colRows
	for i, row := range eRows {
		col := eCols[i]
		r, c := int(row), int(col)
		if r >= len(rowCursor) || c >= len(colCursor) {
			continue
		}

		rowPos := rowCursor[r]
		rowCols[rowPos] = col
		rowCursor[r] = rowPos + 1

		colPos := colCursor[c]
		colRows[colPos] = row
		colCursor[c] = colPos + 1
	}
}

func (dec *inactivateDecoder) rowColumns(row uint32) []uint32 {
	return dec.rowCols[dec.rowStarts[row]:dec.rowStarts[row+1]]
}

func (dec *inactivateDecoder) columnRows(col uint32) []uint32 {
	return dec.colRows[dec.colStarts[col]:dec.colStarts[col+1]]
}

func (dec *inactivateDecoder) sort(arena *matrixArena) {
	// the counting-sort histogram only needs maxCnt+2 buckets, row degrees
	// are far below cols; rowCntOffset entries above maxCnt are never read
	// since row counts only decrease
	rowCnt := dec.rowCnt
	maxCnt := uint32(0)
	for _, c := range rowCnt {
		if c > maxCnt {
			maxCnt = c
		}
	}

	offset := arena.newU32(int(maxCnt) + 2)
	for _, c := range rowCnt {
		offset[c+1]++
	}
	for i := uint32(1); i <= maxCnt+1; i++ {
		offset[i] += offset[i-1]
	}
	dec.rowCntOffset = arena.newU32Dirty(int(dec.rows))
	copy(dec.rowCntOffset, offset)

	// placement fully writes both arrays: bucket cursors tile [0, rows)
	dec.sortedRows = arena.newU32Dirty(int(dec.rows))
	dec.rowPos = arena.newU32Dirty(int(dec.rows))
	for i := uint32(0); i < dec.rows; i++ {
		pos := offset[rowCnt[i]]
		offset[rowCnt[i]]++

		dec.sortedRows[pos] = i
		dec.rowPos[i] = pos
	}
}

func (dec *inactivateDecoder) loop(arena *matrixArena) {
	rowCntOffset, sortedRows, rowCnt := dec.rowCntOffset, dec.sortedRows, dec.rowCnt
	wasCol, wasRow := dec.wasCol, dec.wasRow

	for rowCntOffset[1] != dec.rows {
		row := sortedRows[rowCntOffset[1]]
		col := dec.chooseCol(row)

		cnt := rowCnt[row]
		dec.pCols = append(dec.pCols, col)
		dec.pRows = append(dec.pRows, row)

		if cnt == 1 {
			dec.inactivate(col)
		} else {
			for _, x := range dec.rowColumns(row) {
				if wasCol[x] {
					continue
				}
				if x != col {
					dec.inactiveCols = append(dec.inactiveCols, x)
				}
				dec.inactivate(x)
			}
		}
		wasRow[row] = true
	}
}

func (dec *inactivateDecoder) chooseCol(row uint32) uint32 {
	cnt := dec.rowCnt[row]
	if cnt == 1 {
		return dec.rowXor[row]
	}

	bestCol := uint32(0xFFFFFFFF)
	bestCnt := uint32(0)
	wasCol, colCnt := dec.wasCol, dec.colCnt
	for _, col := range dec.rowColumns(row) {
		if wasCol[col] {
			continue
		}
		c := colCnt[col]
		if bestCol == 0xFFFFFFFF || c < bestCnt {
			bestCol, bestCnt = col, c
		}
	}
	return bestCol
}

func (dec *inactivateDecoder) inactivate(col uint32) {
	dec.wasCol[col] = true

	wasRow, rowPos, rowCnt := dec.wasRow, dec.rowPos, dec.rowCnt
	rowCntOffset, sortedRows, rowXor := dec.rowCntOffset, dec.sortedRows, dec.rowXor
	for _, row := range dec.columnRows(col) {
		if wasRow[row] {
			continue
		}

		// sortedRows[rowPos[row]] == row (inverse permutation invariant),
		// so the swap needs a single load of the other slot
		pos := rowPos[row]
		cnt := rowCnt[row]
		offset := rowCntOffset[cnt]
		other := sortedRows[offset]
		sortedRows[offset] = row
		sortedRows[pos] = other
		rowPos[other] = pos
		rowPos[row] = offset
		rowCntOffset[cnt] = offset + 1
		rowCnt[row] = cnt - 1
		rowXor[row] ^= col
	}
}
