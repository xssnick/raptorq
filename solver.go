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
	res := a.newU32Dirty(n)
	clear(res)
	return res
}

// newU32Dirty returns a possibly stale buffer, for callers that
// fully overwrite it before the first read
func (a *matrixArena) newU32Dirty(n int) []uint32 {
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
	return a.u32s[offset:needed]
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

	// dominated by: D, C and the HDPC scratch (each ~rows*symSz);
	// the sparse blocks and indexes are covered by the L*L/8 slack
	estimate := int((dataRows+3*p._L)*symSz + p._L*p._L/8)
	const maxInitialArena = 64 << 20
	if estimate > maxInitialArena {
		estimate = maxInitialArena
	}
	arena := newMatrixArena(estimate)
	// entries + upper index scale with nnz (~avg LT degree 7 per row + LDPC)
	nnzEstimate := 3*p._B + 3*p._S + 8*uint32(len(symbols))
	typedCap := int(6*nnzEstimate + 16*(rows+p._L+p._P))
	if cap(arena.u32s) < typedCap {
		arena.u32s = make([]uint32, 0, typedCap)
	}
	if cap(arena.bools) < int(rows+2*p._L) {
		arena.bools = make([]bool, 0, int(rows+2*p._L))
	}
	if cap(arena.encodingRows) < len(symbols) {
		arena.encodingRows = make([]encodingRow, 0, len(symbols))
	}
	return arena
}

// createDPermuted builds D with every row already at its permuted position
// (original row r lands at rPerm[r]), so no separate permutation pass is needed.
// The S prefix rows and H suffix rows of the original layout are zero,
// the middle rows carry the symbols.
func (p *raptorParams) createDPermuted(arena *matrixArena, symbols []symbol, rPerm []uint32) *discmath.MatrixGF256 {
	symSz := uint32(len(symbols[0].Data))
	rows := p._S + p._H + uint32(len(symbols))
	d := arena.newGF256Dirty(rows, symSz)

	for r := uint32(0); r < p._S; r++ {
		clear(d.GetRow(rPerm[r]))
	}

	offset := p._S
	for i := range symbols {
		d.RowSet(rPerm[offset], symbols[i].Data)
		offset++
	}

	for r := offset; r < rows; r++ {
		clear(d.GetRow(rPerm[r]))
	}

	return d
}

func (p *raptorParams) Solve(symbols []Symbol) (*discmath.MatrixGF256, error) {
	res, _, err := p.solve(symbols, true, nil)
	return res, err
}

// rowFor optionally overrides calcEncodingRow with a caller cache, nil means direct
func (p *raptorParams) solve(symbols []symbol, keepResult bool, rowFor func(uint32) encodingRow) (*discmath.MatrixGF256, func(), error) {
	arena := p.newSolveArena(symbols)

	if rowFor == nil {
		rowFor = p.calcEncodingRow
	}
	eRows := arena.newEncodingRows(len(symbols))
	for i, symbol := range symbols {
		eRows[i] = rowFor(symbol.ID)
	}

	// exact bound: LDPC/ident nonzeros plus the actual degree of every encoding row
	maxUpperNonZero := 3*p._B + 3*p._S
	for i := range eRows {
		maxUpperNonZero += eRows[i].Size()
	}
	aUpperRows := p._S + uint32(len(eRows))
	upperBuilder := upperMatrixBuilder{
		entries: upperMatrixEntries{
			rows: arena.newU32Dirty(int(maxUpperNonZero)),
			cols: arena.newU32Dirty(int(maxUpperNonZero)),
		},
	}

	// The builder appends cells without a duplicate check: the LT walk is
	// duplicate-free since W is prime, the PI walk since P1 is prime, LDPC2
	// since P >= 2 (see Test_ParamsTableInvariants), and LDPC1 duplicates
	// are filtered right here. The b/a counters track i%S and 1+i/S without
	// per-iteration division; b0+aMod < 2S so one conditional subtract
	// equals the modulo.
	aMod := uint32(1) % p._S
	for i, a, b0 := uint32(0), uint32(1), uint32(0); i < p._B; i++ {
		upperBuilder.set(b0, i)

		b1 := b0 + aMod
		if b1 >= p._S {
			b1 -= p._S
		}
		if b1 != b0 {
			upperBuilder.set(b1, i)
		}

		b2 := b1 + aMod
		if b2 >= p._S {
			b2 -= p._S
		}
		if b2 != b0 && b2 != b1 {
			upperBuilder.set(b2, i)
		}

		b0++
		if b0 == p._S {
			b0 = 0
			a++
			aMod = a % p._S
		}
	}

	// Ident
	for i := uint32(0); i < p._S; i++ {
		upperBuilder.set(i, i+p._B)
	}

	// LDPC 2, j tracks i % p._P
	for i, j := uint32(0), uint32(0); i < p._S; i++ {
		upperBuilder.set(i, j+p._W)
		j++
		if j == p._P {
			j = 0
		}
		upperBuilder.set(i, j+p._W)
	}

	// Encode
	for ri := range eRows {
		eRows[ri].encode(&upperBuilder, uint32(ri), p)
	}

	uSize, rowPermutation, colPermutation := inactivateDecode(arena, aUpperRows, p._L, p._P, upperBuilder.entries)

	dataRows := p._S + p._H + uint32(len(symbols))
	for len(rowPermutation) < int(dataRows) {
		rowPermutation = append(rowPermutation, uint32(len(rowPermutation)))
	}

	rPermutation := inversePermutationArena(arena, rowPermutation)
	cPermutation := inversePermutationArena(arena, colPermutation)

	d := p.createDPermuted(arena, symbols, rPermutation)

	// smallA/smallD are allocated combined, their upper/lower halves are
	// zero-copy row-range views; smallA must be fully zeroed (its blocks are
	// filled sparsely), smallD is fully overwritten from d blocks
	symSz := d.ColsNum()
	smallA := arena.newGF256(aUpperRows-uSize+p._H, p._L-uSize)
	smallAUpper := arena.newGF256FromData(aUpperRows-uSize, smallA.Cols, smallA.Data[:(aUpperRows-uSize)*smallA.Cols])
	smallALower := arena.newGF256FromData(p._H, smallA.Cols, smallA.Data[(aUpperRows-uSize)*smallA.Cols:])

	e, gLeft, upperIndex := buildPermutedUpperArena(arena, upperBuilder.entries, rPermutation, cPermutation, aUpperRows, p._L, uSize, smallAUpper)

	// c rows are placed directly at their final (inverse column permutation)
	// positions, so no permutation pass is needed at the end: assembly row r
	// of the solution lives at c row colPermutation[r].
	var c *discmath.MatrixGF256
	if keepResult {
		c = discmath.NewMatrixGF256(p._L, d.ColsNum())
	} else {
		c = arena.newGF256Dirty(p._L, d.ColsNum())
	}
	for r := uint32(0); r < uSize; r++ {
		c.RowSet(colPermutation[r], d.GetRow(r))
	}

	// Make U Identity matrix and calculate E and D_upper.
	for i := uint32(0); i < uSize; i++ {
		rows := upperIndex.colRowsFor(i)
		if len(rows) == 0 {
			continue
		}
		eRow := e.GetRow(i)
		dRow := d.GetRow(i)
		for _, row := range rows {
			if row == i {
				continue
			}

			e.RowAdd(row, eRow)
			d.RowAdd(row, dRow)
		}
	}

	// the HDPC (a, b) random pairs are identical for every hdpcMultiply call
	// in this solve, compute them once; a+r+1 < 2H so one conditional
	// subtract equals % H
	tRows := p._KPadded + p._S
	hdpcAB := arena.newU32Dirty(int(2 * (tRows - 1)))
	for col := uint32(0); col+1 < tRows; col++ {
		a := random(col+1, 6, p._H)
		b := a + random(col+1, 7, p._H-1) + 1
		if b >= p._H {
			b -= p._H
		}
		hdpcAB[2*col] = a
		hdpcAB[2*col+1] = b
	}

	// rows of t not covered by colPermutation[:covered] are exactly the
	// in-range entries of the permutation tail, colPermutation is a
	// bijection on [0, L) and tRows < L
	clearHDPCGaps := func(t *discmath.MatrixGF256, covered uint32) {
		for _, row := range colPermutation[covered:] {
			if row < tRows {
				clear(t.GetRow(row))
			}
		}
	}

	hdpcMul := func(m *discmath.MatrixGF256) *discmath.MatrixGF256 {
		t := arena.newGF256Dirty(tRows, m.ColsNum())
		for i := uint32(0); i < m.RowsNum(); i++ {
			t.RowSet(colPermutation[i], m.GetRow(i))
		}
		clearHDPCGaps(t, m.RowsNum())
		return p.hdpcMultiply(arena, t, hdpcAB)
	}

	// same as hdpcMul but expands GF2 bit rows straight into the scatter
	// destination, skipping the intermediate dense matrix
	hdpcMulGF2 := func(m *discmath.PlainMatrixGF2) *discmath.MatrixGF256 {
		t := arena.newGF256Dirty(tRows, m.ColsNum())
		for i := uint32(0); i < m.RowsNum(); i++ {
			m.RowToGF256(i, t.GetRow(colPermutation[i]))
		}
		clearHDPCGaps(t, m.RowsNum())
		return p.hdpcMultiply(arena, t, hdpcAB)
	}

	smallAUpper.Add(plainGF2ToGF256Arena(arena, mulGF2Arena(arena, e, gLeft)))

	// small A lower identity part
	for i := uint32(1); i <= p._H; i++ {
		smallALower.Set(smallALower.RowsNum()-i, smallALower.ColsNum()-i, 1)
	}

	// calculate HDPC right and set it into small A lower
	t := arena.newGF256(tRows, tRows-uSize)
	for i := uint32(0); i < t.ColsNum(); i++ {
		t.Set(colPermutation[i+t.RowsNum()-t.ColsNum()], i, 1)
	}
	hdpcRight := p.hdpcMultiply(arena, t, hdpcAB)
	smallALower.SetFrom(hdpcRight, 0, 0)

	// ALower += hdpc(E)
	smallALower.Add(hdpcMulGF2(e))

	// dUpper is a read-only view of the first uSize rows of d, no copy needed
	dUpper := arena.newGF256FromData(uSize, symSz, d.Data[:uSize*d.Cols])

	smallD := arena.newGF256Dirty(aUpperRows-uSize+p._H, symSz)
	smallDUpper := arena.newGF256FromData(aUpperRows-uSize, symSz, smallD.Data[:(aUpperRows-uSize)*symSz])
	smallDLower := arena.newGF256FromData(p._H, symSz, smallD.Data[(aUpperRows-uSize)*symSz:])

	smallDUpper.SetFromBlock(d, uSize, 0, smallDUpper.RowsNum(), smallDUpper.ColsNum(), 0, 0)
	mulSparseInto(smallDUpper, dUpper, gLeft)

	smallDLower.SetFromBlock(d, aUpperRows, 0, smallDLower.RowsNum(), smallDLower.ColsNum(), 0, 0)
	smallDLower.Add(hdpcMul(dUpper))

	// the solution row r of the elimination lives at smallC.GetRow(gaussPerm[r])
	gaussPerm := arena.newU32Dirty(int(smallA.RowsNum()))
	smallC, err := discmath.GaussianElimination(smallA, smallD, gaussPerm)
	if err != nil {
		arena.release()
		if errors.Is(err, discmath.ErrNotSolvable) {
			return nil, nil, errNotEnoughSymbols
		}
		return nil, nil, fmt.Errorf("failed to calc gauss elimination: %w", err)
	}

	for r := uint32(0); r < c.RowsNum()-uSize; r++ {
		c.RowSet(colPermutation[uSize+r], smallC.GetRow(gaussPerm[r]))
	}
	for row := uint32(0); row < uSize; row++ {
		cRow := c.GetRow(colPermutation[row])
		for _, col := range upperIndex.rowColsFor(row) {
			if col == row {
				continue
			}
			discmath.OctVecAdd(cRow, c.GetRow(colPermutation[col]))
		}
	}

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
	entries upperMatrixEntries
}

// set records a nonzero cell, the caller must never pass the same cell twice
func (b *upperMatrixBuilder) set(row, col uint32) {
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
	// all four arrays are fully written before the first read: the starts by
	// the prefix sums below, the cols/rows by the caller's cursor scatter
	idx := upperSparseIndex{
		rowStarts: arena.newU32Dirty(int(rowsLimit + 1)),
		rowCols:   arena.newU32Dirty(int(rowNNZ)),
		colStarts: arena.newU32Dirty(int(colIndexLimit + 1)),
		colRows:   arena.newU32Dirty(int(colNNZ)),
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
	// mut is an exact permutation, so every index is written before any read
	res := arena.newU32Dirty(len(mut))
	for i, u := range mut {
		res[u] = uint32(i)
	}
	return res
}

// buildPermutedUpperArena splits the permuted sparse upper matrix directly into the
// blocks the solver needs, without materializing the dense permuted matrix:
//
//	| U (idx only) | E (GF2)         |   rows < uSize
//	| gLeft        | smallAUpper bin |   rows >= uSize
//
// It also builds the sparse row/col index of the top uSize rows (U and E parts).
// smallAUpper is filled by the caller-provided zeroed view. The permuted upper
// coordinates are compacted in place into entries.rows/cols, which are dead
// after this call: at iteration i the write index rowNNZ <= i, and the cell at
// i was already consumed at the top of the iteration.
func buildPermutedUpperArena(arena *matrixArena, entries upperMatrixEntries, rPerm, cPerm []uint32, rows, cols, uSize uint32, smallAUpper *discmath.MatrixGF256) (*discmath.PlainMatrixGF2, *discmath.MatrixGF256, upperSparseIndex) {
	if !entries.valid() {
		panic("raptorq: upper matrix entries overflow")
	}

	e := arena.newGF2(uSize, cols-uSize)
	gLeft := arena.newGF256(rows-uSize, uSize)

	rowCounts := arena.newU32(int(uSize))
	colCounts := arena.newU32(int(uSize))
	upperRows := entries.rows[:entries.n]
	upperCols := entries.cols[:entries.n]

	rowNNZ := uint32(0)
	colNNZ := uint32(0)
	for i := uint32(0); i < entries.n; i++ {
		dstRow := rPerm[entries.rows[i]]
		dstCol := cPerm[entries.cols[i]]
		if dstRow < uSize {
			upperRows[rowNNZ] = dstRow
			upperCols[rowNNZ] = dstCol
			rowCounts[dstRow]++
			rowNNZ++
			if dstCol < uSize {
				colCounts[dstCol]++
				colNNZ++
			} else {
				e.Set(dstRow, dstCol-uSize)
			}
		} else if dstCol < uSize {
			gLeft.Set(dstRow-uSize, dstCol, 1)
		} else {
			smallAUpper.Set(dstRow-uSize, dstCol-uSize, 1)
		}
	}

	idx := newUpperSparseIndexFromCounts(arena, rowCounts, colCounts, rowNNZ, colNNZ, uSize, uSize)
	rowCursor := arena.newU32Dirty(int(uSize))
	colCursor := arena.newU32Dirty(int(uSize))
	copy(rowCursor, idx.rowStarts[:uSize])
	copy(colCursor, idx.colStarts[:uSize])

	for i := uint32(0); i < rowNNZ; i++ {
		dstRow := upperRows[i]
		dstCol := upperCols[i]

		rowPos := rowCursor[dstRow]
		idx.rowCols[rowPos] = dstCol
		rowCursor[dstRow] = rowPos + 1

		if dstCol < uSize {
			colPos := colCursor[dstCol]
			idx.colRows[colPos] = dstRow
			colCursor[dstCol] = colPos + 1
		}
	}

	return e, gLeft, idx
}

func mulSparseInto(dst, m, s *discmath.MatrixGF256) {
	for row := uint32(0); row < s.Rows; row++ {
		rowData := s.GetRow(row)
		dstRow := dst.GetRow(row)
		for col, val := range rowData {
			if val != 0 {
				discmath.OctVecAdd(dstRow, m.GetRow(uint32(col)))
			}
		}
	}
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
