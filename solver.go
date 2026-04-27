package raptorq

import (
	"errors"
	"fmt"
	"sync"

	"github.com/xssnick/raptorq/internal/discmath"
)

var errNotEnoughSymbols = errors.New("not enough symbols")

var ErrNotEnoughSymbols = errNotEnoughSymbols

type matrixArena struct {
	chunks        [][]byte
	chunkIdx      int
	cur           []byte
	matrices      []discmath.MatrixGF256
	plainMatrices []discmath.PlainMatrixGF2
	encodingRows  []encodingRow
	u32s          []uint32
	bools         []bool
}

var matrixArenaPool sync.Pool

func newMatrixArena(initialCap int) *matrixArena {
	if initialCap < 4096 {
		initialCap = 4096
	}

	if v := matrixArenaPool.Get(); v != nil {
		arena := v.(*matrixArena)
		arena.reset(initialCap)
		return arena
	}

	arena := &matrixArena{}
	arena.reset(initialCap)
	return arena
}

func (a *matrixArena) reset(initialCap int) {
	if len(a.chunks) == 0 || cap(a.chunks[0]) < initialCap {
		a.chunks = [][]byte{make([]byte, initialCap)}
	} else {
		a.chunks[0] = a.chunks[0][:cap(a.chunks[0])]
	}
	a.chunkIdx = 0
	a.cur = a.chunks[0][:0]

	if cap(a.matrices) < 128 {
		a.matrices = make([]discmath.MatrixGF256, 0, 128)
	} else {
		a.matrices = a.matrices[:0]
	}
	if cap(a.plainMatrices) < 16 {
		a.plainMatrices = make([]discmath.PlainMatrixGF2, 0, 16)
	} else {
		a.plainMatrices = a.plainMatrices[:0]
	}
	if cap(a.encodingRows) < 128 {
		a.encodingRows = make([]encodingRow, 0, 128)
	} else {
		a.encodingRows = a.encodingRows[:0]
	}
	a.u32s = a.u32s[:0]
	a.bools = a.bools[:0]
}

func (a *matrixArena) release() {
	const maxRetainedArena = 64 << 20
	total := 0
	for _, chunk := range a.chunks {
		total += cap(chunk)
	}
	if len(a.chunks) == 0 || total > maxRetainedArena {
		*a = matrixArena{}
		return
	}
	matrixArenaPool.Put(a)
}

func (a *matrixArena) newBytes(size int) []byte {
	if size == 0 {
		return nil
	}
	if cap(a.cur)-len(a.cur) < size {
		chunkSize := cap(a.cur) * 2
		if chunkSize < size {
			chunkSize = size
		}
		a.chunkIdx++
		if a.chunkIdx < len(a.chunks) && cap(a.chunks[a.chunkIdx]) >= size {
			a.cur = a.chunks[a.chunkIdx][:0]
		} else {
			chunk := make([]byte, chunkSize)
			if a.chunkIdx < len(a.chunks) {
				a.chunks[a.chunkIdx] = chunk
				a.chunks = a.chunks[:a.chunkIdx+1]
			} else {
				a.chunks = append(a.chunks, chunk)
			}
			a.cur = chunk[:0]
		}
	}

	offset := len(a.cur)
	a.cur = a.cur[:offset+size]
	return a.cur[offset : offset+size]
}

func (a *matrixArena) newZeroedBytes(size int) []byte {
	data := a.newBytes(size)
	clear(data)
	return data
}

func (a *matrixArena) newGF256(rows, cols uint32) *discmath.MatrixGF256 {
	size := int(rows) * int(cols)
	data := a.newZeroedBytes(size)
	return a.newGF256FromData(rows, cols, data)
}

func (a *matrixArena) newGF256Dirty(rows, cols uint32) *discmath.MatrixGF256 {
	size := int(rows) * int(cols)
	data := a.newBytes(size)
	return a.newGF256FromData(rows, cols, data)
}

func (a *matrixArena) newGF256FromData(rows, cols uint32, data []byte) *discmath.MatrixGF256 {
	if len(a.matrices) == cap(a.matrices) {
		return &discmath.MatrixGF256{
			Rows: rows,
			Cols: cols,
			Data: data,
		}
	}

	idx := len(a.matrices)
	a.matrices = a.matrices[:idx+1]
	m := &a.matrices[idx]
	*m = discmath.MatrixGF256{
		Rows: rows,
		Cols: cols,
		Data: data,
	}
	return m
}

func (a *matrixArena) newU32(n int) []uint32 {
	if n == 0 {
		return nil
	}
	offset := len(a.u32s)
	needed := offset + n
	if needed > cap(a.u32s) {
		nextCap := cap(a.u32s) * 2
		if nextCap < needed {
			nextCap = needed
		}
		next := make([]uint32, offset, nextCap)
		copy(next, a.u32s)
		a.u32s = next
	}
	a.u32s = a.u32s[:needed]
	res := a.u32s[offset:needed]
	clear(res)
	return res
}

func (a *matrixArena) newBool(n int) []bool {
	if n == 0 {
		return nil
	}
	offset := len(a.bools)
	needed := offset + n
	if needed > cap(a.bools) {
		nextCap := cap(a.bools) * 2
		if nextCap < needed {
			nextCap = needed
		}
		next := make([]bool, offset, nextCap)
		copy(next, a.bools)
		a.bools = next
	}
	a.bools = a.bools[:needed]
	res := a.bools[offset:needed]
	clear(res)
	return res
}

func (a *matrixArena) newGF2(rows, cols uint32) *discmath.PlainMatrixGF2 {
	data := a.newBytes(discmath.PlainMatrixGF2DataSize(rows, cols))
	if len(a.plainMatrices) == cap(a.plainMatrices) {
		m := &discmath.PlainMatrixGF2{}
		discmath.InitPlainMatrixGF2(m, rows, cols, data)
		return m
	}

	idx := len(a.plainMatrices)
	a.plainMatrices = a.plainMatrices[:idx+1]
	m := &a.plainMatrices[idx]
	discmath.InitPlainMatrixGF2(m, rows, cols, data)
	return m
}

func (a *matrixArena) newEncodingRows(n int) []encodingRow {
	if n == 0 {
		return nil
	}
	offset := len(a.encodingRows)
	needed := offset + n
	if needed > cap(a.encodingRows) {
		nextCap := cap(a.encodingRows) * 2
		if nextCap < needed {
			nextCap = needed
		}
		next := make([]encodingRow, offset, nextCap)
		copy(next, a.encodingRows)
		a.encodingRows = next
	}
	a.encodingRows = a.encodingRows[:needed]
	return a.encodingRows[offset:needed]
}

func (p *raptorParams) newSolveArena(symbols []symbol) *matrixArena {
	symSz := uint32(len(symbols[0].Data))
	rows := p._S + uint32(len(symbols))
	dataRows := p._S + p._H + uint32(len(symbols))

	estimate := int(dataRows*symSz + rows*p._L + 4*p._L*symSz + p._L*p._L/8)
	const maxInitialArena = 64 << 20
	if estimate > maxInitialArena {
		estimate = maxInitialArena
	}
	arena := newMatrixArena(estimate)
	typedCap := int(16 * (rows + p._L + p._P))
	if cap(arena.u32s) < typedCap {
		arena.u32s = make([]uint32, 0, typedCap)
	}
	if cap(arena.bools) < int(rows+p._L) {
		arena.bools = make([]bool, 0, int(rows+p._L))
	}
	if cap(arena.encodingRows) < len(symbols) {
		arena.encodingRows = make([]encodingRow, 0, len(symbols))
	}
	return arena
}

func (p *raptorParams) createD(arena *matrixArena, symbols []symbol) *discmath.MatrixGF256 {
	symSz := uint32(len(symbols[0].Data))
	d := arena.newGF256(p._S+p._H+uint32(len(symbols)), symSz)

	offset := p._S
	for i := range symbols {
		d.RowSet(offset, symbols[i].Data)
		offset++
	}

	return d
}

func (p *raptorParams) Solve(symbols []Symbol) (*discmath.MatrixGF256, error) {
	res, _, err := p.solve(symbols, true)
	return res, err
}

func (p *raptorParams) solve(symbols []symbol, keepResult bool) (*discmath.MatrixGF256, func(), error) {
	arena := p.newSolveArena(symbols)
	d := p.createD(arena, symbols)

	eRows := arena.newEncodingRows(len(symbols))
	for i, symbol := range symbols {
		eRows[i] = p.calcEncodingRow(symbol.ID)
	}

	maxUpperNonZero := 3*p._B + 3*p._S + 33*uint32(len(eRows))
	aUpperRows := p._S + uint32(len(eRows))
	aUpper := arena.newGF256(aUpperRows, p._L)
	upperBuilder := upperMatrixBuilder{
		m: aUpper,
		entries: upperMatrixEntries{
			rows: arena.newU32(int(maxUpperNonZero)),
			cols: arena.newU32(int(maxUpperNonZero)),
		},
	}

	// LDPC 1
	for i := uint32(0); i < p._B; i++ {
		a := 1 + i/p._S

		b := i % p._S
		upperBuilder.set(b, i)
		b = (b + a) % p._S
		upperBuilder.set(b, i)

		b = (b + a) % p._S
		upperBuilder.set(b, i)

	}

	// Ident
	for i := uint32(0); i < p._S; i++ {
		upperBuilder.set(i, i+p._B)
	}

	// LDPC 2
	for i := uint32(0); i < p._S; i++ {
		upperBuilder.set(i, (i%p._P)+p._W)
		upperBuilder.set(i, ((i+1)%p._P)+p._W)
	}

	// Encode
	for ri := range eRows {
		eRows[ri].encode(&upperBuilder, uint32(ri), p)
	}

	uSize, rowPermutation, colPermutation := inactivateDecode(arena, aUpper, p._P, upperBuilder.entries)

	for len(rowPermutation) < int(d.RowsNum()) {
		rowPermutation = append(rowPermutation, uint32(len(rowPermutation)))
	}

	d = applyPermutationArena(arena, d, rowPermutation)

	rPermutation := inversePermutationArena(arena, rowPermutation)
	cPermutation := inversePermutationArena(arena, colPermutation)
	aUpper, upperIndex := applyRCPermutationAndUpperIndexArena(arena, aUpper, upperBuilder.entries, rPermutation, cPermutation, uSize, uSize, maxUpperNonZero)

	e := toGF2Arena(arena, aUpper, 0, uSize, uSize, p._L-uSize)

	var c *discmath.MatrixGF256
	if keepResult {
		c = discmath.NewMatrixGF256(aUpper.ColsNum(), d.ColsNum())
	} else {
		c = arena.newGF256Dirty(aUpper.ColsNum(), d.ColsNum())
	}
	c.SetFromBlock(d, 0, 0, uSize, d.ColsNum(), 0, 0)

	// Make U Identity matrix and calculate E and D_upper.
	for i := uint32(0); i < uSize; i++ {
		for _, row := range upperIndex.colRowsFor(i) {
			if row == i {
				continue
			}

			e.RowAdd(row, e.GetRow(i))
			d.RowAdd(row, d.GetRow(i))
		}
	}

	hdpcMul := func(m *discmath.MatrixGF256) *discmath.MatrixGF256 {
		t := arena.newGF256(p._KPadded+p._S, m.ColsNum())
		for i := uint32(0); i < m.RowsNum(); i++ {
			t.RowSet(colPermutation[i], m.GetRow(i))
		}
		return p.hdpcMultiply(arena, t)
	}

	gLeft := getBlockArena(arena, aUpper, uSize, 0, aUpper.RowsNum()-uSize, uSize)

	smallAUpper := arena.newGF256(aUpper.RowsNum()-uSize, aUpper.ColsNum()-uSize)

	setBinaryBlock(smallAUpper, aUpper, uSize, uSize, smallAUpper.RowsNum(), smallAUpper.ColsNum())

	smallAUpper = smallAUpper.Add(plainGF2ToGF256Arena(arena, mulGF2Arena(arena, e, gLeft)))

	// calculate small A lower
	smallALower := arena.newGF256(p._H, aUpper.ColsNum()-uSize)
	for i := uint32(1); i <= p._H; i++ {
		smallALower.Set(smallALower.RowsNum()-i, smallALower.ColsNum()-i, 1)
	}

	// calculate HDPC right and set it into small A lower
	t := arena.newGF256(p._KPadded+p._S, p._KPadded+p._S-uSize)
	for i := uint32(0); i < t.ColsNum(); i++ {
		t.Set(colPermutation[i+t.RowsNum()-t.ColsNum()], i, 1)
	}
	hdpcRight := p.hdpcMultiply(arena, t)
	smallALower.SetFrom(hdpcRight, 0, 0)

	// ALower += hdpc(E)
	smallALower = smallALower.Add(hdpcMul(plainGF2ToGF256Arena(arena, e)))

	dUpper := arena.newGF256Dirty(uSize, d.ColsNum())
	dUpper.SetFromBlock(d, 0, 0, dUpper.RowsNum(), dUpper.ColsNum(), 0, 0)

	smallDUpper := arena.newGF256Dirty(aUpper.RowsNum()-uSize, d.ColsNum())
	smallDUpper.SetFromBlock(d, uSize, 0, smallDUpper.RowsNum(), smallDUpper.ColsNum(), 0, 0)
	smallDUpper = smallDUpper.Add(mulSparseArena(arena, dUpper, gLeft))

	smallDLower := arena.newGF256Dirty(p._H, d.ColsNum())
	smallDLower.SetFromBlock(d, aUpper.RowsNum(), 0, smallDLower.RowsNum(), smallDLower.ColsNum(), 0, 0)
	smallDLower = smallDLower.Add(hdpcMul(dUpper))

	// combine small A
	smallA := arena.newGF256Dirty(smallAUpper.RowsNum()+smallALower.RowsNum(), smallAUpper.ColsNum())
	smallA.SetFrom(smallAUpper, 0, 0)
	smallA.SetFrom(smallALower, smallAUpper.RowsNum(), 0)

	// combine small D
	smallD := arena.newGF256Dirty(smallDUpper.RowsNum()+smallDLower.RowsNum(), smallDUpper.ColsNum())
	smallD.SetFrom(smallDUpper, 0, 0)
	smallD.SetFrom(smallDLower, smallDUpper.RowsNum(), 0)

	smallC, err := discmath.GaussianElimination(
		smallA,
		smallD,
		arena.newU32(int(smallA.RowsNum())),
		arena.newZeroedBytes(int(smallD.RowsNum()+smallD.ColsNum())),
	)
	if err != nil {
		arena.release()
		if errors.Is(err, discmath.ErrNotSolvable) {
			return nil, nil, errNotEnoughSymbols
		}
		return nil, nil, fmt.Errorf("failed to calc gauss elimination: %w", err)
	}

	c.SetFromBlock(smallC, 0, 0, c.RowsNum()-uSize, c.ColsNum(), uSize, 0)
	for row := uint32(0); row < uSize; row++ {
		for _, col := range upperIndex.rowColsFor(row) {
			if col == row {
				continue
			}
			c.RowAdd(row, c.GetRow(col))
		}
	}

	c = applyPermutationArena(arena, c, inversePermutationArena(arena, colPermutation))
	if keepResult {
		arena.release()
		return c, nil, nil
	}
	return c, arena.release, nil
}

type upperSparseIndex struct {
	rowStarts []uint32
	rowCols   []uint32
	colStarts []uint32
	colRows   []uint32
}

type upperMatrixEntries struct {
	rows     []uint32
	cols     []uint32
	n        uint32
	overflow bool
}

func (e upperMatrixEntries) valid() bool {
	return !e.overflow && e.n <= uint32(len(e.rows)) && e.n <= uint32(len(e.cols))
}

type upperMatrixBuilder struct {
	m       *discmath.MatrixGF256
	entries upperMatrixEntries
}

func (b *upperMatrixBuilder) set(row, col uint32) {
	if b.m.Get(row, col) != 0 {
		return
	}

	b.m.Set(row, col, 1)
	if b.entries.overflow {
		return
	}
	if b.entries.n >= uint32(len(b.entries.rows)) {
		b.entries.overflow = true
		return
	}

	b.entries.rows[b.entries.n] = row
	b.entries.cols[b.entries.n] = col
	b.entries.n++
}

func newUpperSparseIndexFromCounts(arena *matrixArena, rowCounts, colCounts []uint32, rowNNZ, colNNZ, rowsLimit, colIndexLimit uint32) upperSparseIndex {
	idx := upperSparseIndex{
		rowStarts: arena.newU32(int(rowsLimit + 1)),
		rowCols:   arena.newU32(int(rowNNZ)),
		colStarts: arena.newU32(int(colIndexLimit + 1)),
		colRows:   arena.newU32(int(colNNZ)),
	}

	offset := uint32(0)
	for row := uint32(0); row < rowsLimit; row++ {
		idx.rowStarts[row] = offset
		offset += rowCounts[row]
	}
	idx.rowStarts[rowsLimit] = offset

	offset = 0
	for col := uint32(0); col < colIndexLimit; col++ {
		idx.colStarts[col] = offset
		offset += colCounts[col]
	}
	idx.colStarts[colIndexLimit] = offset

	return idx
}

func (idx upperSparseIndex) rowColsFor(row uint32) []uint32 {
	return idx.rowCols[idx.rowStarts[row]:idx.rowStarts[row+1]]
}

func (idx upperSparseIndex) colRowsFor(col uint32) []uint32 {
	return idx.colRows[idx.colStarts[col]:idx.colStarts[col+1]]
}

func inversePermutationArena(arena *matrixArena, mut []uint32) []uint32 {
	res := arena.newU32(len(mut))
	for i, u := range mut {
		res[u] = uint32(i)
	}
	return res
}

func getBlockArena(arena *matrixArena, m *discmath.MatrixGF256, rowOffset, colOffset, rowSize, colSize uint32) *discmath.MatrixGF256 {
	res := arena.newGF256Dirty(rowSize, colSize)
	res.SetFromBlock(m, rowOffset, colOffset, rowSize, colSize, 0, 0)
	return res
}

func setBinaryBlock(dst, src *discmath.MatrixGF256, rowOffset, colOffset, rowSize, colSize uint32) {
	for row := uint32(0); row < rowSize; row++ {
		srcRow := src.GetRow(row + rowOffset)[colOffset : colOffset+colSize]
		dstRow := dst.GetRow(row)
		for col, val := range srcRow {
			if val != 0 {
				dstRow[col] = 1
			}
		}
	}
}

func toGF2Arena(arena *matrixArena, m *discmath.MatrixGF256, rowFrom, colFrom, rowSize, colSize uint32) *discmath.PlainMatrixGF2 {
	mGF2 := arena.newGF2(rowSize, colSize)
	rowTo := rowFrom + rowSize
	colTo := colFrom + colSize
	for row := rowFrom; row < rowTo; row++ {
		rowData := m.GetRow(row)[colFrom:colTo]
		for col, val := range rowData {
			if val != 0 {
				mGF2.Set(row-rowFrom, uint32(col))
			}
		}
	}
	return mGF2
}

func applyRCPermutationAndUpperIndexArena(arena *matrixArena, m *discmath.MatrixGF256, entries upperMatrixEntries, rPerm, cPerm []uint32, rowsLimit, colIndexLimit, maxUpperNonZero uint32) (*discmath.MatrixGF256, upperSparseIndex) {
	if !entries.valid() {
		panic("raptorq: upper matrix entries overflow")
	}

	res := arena.newGF256(m.RowsNum(), m.ColsNum())
	rowCounts := arena.newU32(int(rowsLimit))
	colCounts := arena.newU32(int(colIndexLimit))
	upperRows := arena.newU32(int(maxUpperNonZero))
	upperCols := arena.newU32(int(maxUpperNonZero))

	rowNNZ := uint32(0)
	colNNZ := uint32(0)
	for i := uint32(0); i < entries.n; i++ {
		row := entries.rows[i]
		col := entries.cols[i]
		dstRow := rPerm[row]
		dstCol := cPerm[col]
		res.Data[dstRow*res.Cols+dstCol] = 1
		if dstRow < rowsLimit {
			if rowNNZ >= uint32(len(upperRows)) {
				panic("raptorq: upper sparse index overflow")
			}
			upperRows[rowNNZ] = dstRow
			upperCols[rowNNZ] = dstCol
			rowCounts[dstRow]++
			rowNNZ++
			if dstCol < colIndexLimit {
				colCounts[dstCol]++
				colNNZ++
			}
		}
	}

	idx := newUpperSparseIndexFromCounts(arena, rowCounts, colCounts, rowNNZ, colNNZ, rowsLimit, colIndexLimit)
	rowCursor := arena.newU32(int(rowsLimit))
	colCursor := arena.newU32(int(colIndexLimit))
	copy(rowCursor, idx.rowStarts[:rowsLimit])
	copy(colCursor, idx.colStarts[:colIndexLimit])

	for i := uint32(0); i < rowNNZ; i++ {
		dstRow := upperRows[i]
		dstCol := upperCols[i]

		rowPos := rowCursor[dstRow]
		idx.rowCols[rowPos] = dstCol
		rowCursor[dstRow]++

		if dstCol < colIndexLimit {
			colPos := colCursor[dstCol]
			idx.colRows[colPos] = dstRow
			colCursor[dstCol]++
		}
	}

	return res, idx
}

func applyPermutationArena(arena *matrixArena, m *discmath.MatrixGF256, permutation []uint32) *discmath.MatrixGF256 {
	if len(permutation) != int(m.Rows) || m.Rows <= 1 {
		res := arena.newGF256Dirty(m.RowsNum(), m.ColsNum())
		for row := uint32(0); row < m.RowsNum(); row++ {
			res.RowSet(row, m.GetRow(permutation[row]))
		}
		return res
	}

	scratch := arena.newZeroedBytes(int(m.Rows) + int(m.Cols))
	visited := scratch[:m.Rows]
	tmp := scratch[m.Rows:]
	return m.ApplyPermutationInPlaceScratch(permutation, visited, tmp)
}

func mulSparseArena(arena *matrixArena, m, s *discmath.MatrixGF256) *discmath.MatrixGF256 {
	mg := arena.newGF256(s.RowsNum(), m.ColsNum())
	for row := uint32(0); row < s.Rows; row++ {
		rowData := s.GetRow(row)
		for col, val := range rowData {
			if val != 0 {
				mg.RowAdd(row, m.GetRow(uint32(col)))
			}
		}
	}
	return mg
}

func mulGF2Arena(arena *matrixArena, m *discmath.PlainMatrixGF2, s *discmath.MatrixGF256) *discmath.PlainMatrixGF2 {
	return m.MulTo(s, arena.newGF2(s.RowsNum(), m.ColsNum()))
}

func plainGF2ToGF256Arena(arena *matrixArena, m *discmath.PlainMatrixGF2) *discmath.MatrixGF256 {
	mg := arena.newGF256Dirty(m.RowsNum(), m.ColsNum())
	for row := uint32(0); row < m.RowsNum(); row++ {
		m.RowToGF256(row, mg.GetRow(row))
	}
	return mg
}
