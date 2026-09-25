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

	// dominated by the symbol-sized rows: U^-1 D_upper and smallD
	// (together dataRows) and C (L, only in the arena for a decode, but
	// counted either way so an encoder's pooled arena also fits a decode);
	// the sparse blocks and indexes are covered by the L*L/8 slack
	estimate := int((dataRows+p._L)*symSz + p._L*p._L/8)
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

	rPermutation := inversePermutationArena(arena, rowPermutation)
	cPermutation := inversePermutationArena(arena, colPermutation)

	// D is never built: the D row of permuted upper row r is the payload of
	// the symbol behind it (nil for an all-zero LDPC row), which the row
	// kernels read directly as their first source
	symSz := uint32(len(symbols[0].Data))
	dRow := func(r uint32) []byte {
		if o := rowPermutation[r]; o >= p._S {
			return symbols[o-p._S].Data
		}
		return nil
	}

	// smallA is allocated combined, its upper/lower halves are zero-copy
	// row-range views; it must be fully zeroed (its blocks are filled sparsely)
	smallA := arena.newGF256(aUpperRows-uSize+p._H, p._L-uSize)
	smallAUpper := arena.newGF256FromData(aUpperRows-uSize, smallA.Cols, smallA.Data[:(aUpperRows-uSize)*smallA.Cols])
	smallALower := arena.newGF256FromData(p._H, smallA.Cols, smallA.Data[(aUpperRows-uSize)*smallA.Cols:])

	e, gIdx, upperIndex := buildPermutedUpperArena(arena, upperBuilder.entries, rPermutation, cPermutation, aUpperRows, p._L, uSize, smallAUpper)

	// Forward substitution through U, which is unit lower-triangular (see
	// buildPermutedUpperArena): row r of U^-1 [E | D] is [E | D]_r ^ the XOR
	// of the finished rows c < r with U[r][c] = 1, each row is written once
	dUpper := arena.newGF256Dirty(uSize, symSz)
	for r := uint32(0); r < uSize; r++ {
		cols := upperIndex.rowUFor(r)
		for _, col := range cols {
			e.RowAdd(r, e.GetRow(col))
		}
		discmath.OctVecAddRowsTo(dUpper.GetRow(r), dRow(r), dUpper, cols)
	}

	// the HDPC (a, b) random pairs are identical for both hdpcStream calls
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

	lower := aUpperRows - uSize
	nonU := p._L - uSize
	scratch := arena.newBytes(int(nonU))
	hdpcAcc := arena.newBytes(int(max(nonU, symSz)))

	// smallAUpper += gLeft * E', one GF2 row accumulation per gLeft cell
	gE := arena.newGF2(lower, nonU)
	for l := uint32(0); l < lower; l++ {
		for _, col := range gIdx.rowColsFor(l) {
			gE.RowAdd(l, e.GetRow(col))
		}
		gE.RowToGF256(l, scratch)
		smallAUpper.RowAdd(l, scratch)
	}

	// smallALower = HDPC * [E' ; unit columns] + [0 | I_H]. Row col of the
	// HDPC input is the column at permuted position inv: E' row inv for a U
	// column, else the unit vector of smallA column inv-uSize (the HDPC
	// block right of U). Original columns < K'+S never sit in the last H
	// permuted positions (the identity part), so inv-uSize < nonU-H.
	p.hdpcStream(smallALower, hdpcAcc[:nonU], hdpcAB, func(col uint32) []byte {
		if inv := cPermutation[col]; inv < uSize {
			e.RowToGF256(inv, scratch)
		} else {
			clear(scratch)
			scratch[inv-uSize] = 1
		}
		return scratch
	})
	for i := uint32(1); i <= p._H; i++ {
		smallALower.Data[(p._H-i)*nonU+nonU-i] ^= 1
	}

	smallD := arena.newGF256Dirty(lower+p._H, symSz)
	smallDUpper := arena.newGF256FromData(lower, symSz, smallD.Data[:lower*symSz])
	smallDLower := arena.newGF256FromData(p._H, symSz, smallD.Data[lower*symSz:])

	// smallDUpper = D_lower ^ gLeft * U^-1 D_upper
	for l := uint32(0); l < lower; l++ {
		discmath.OctVecAddRowsTo(smallDUpper.GetRow(l), dRow(uSize+l), dUpper, gIdx.rowColsFor(l))
	}

	// smallDLower = HDPC * U^-1 D_upper, the D rows of the HDPC constraints are zero
	p.hdpcStream(smallDLower, hdpcAcc[:symSz], hdpcAB, func(col uint32) []byte {
		if inv := cPermutation[col]; inv < uSize {
			return dUpper.GetRow(inv)
		}
		return nil
	})

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

	// c rows are placed directly at their final (inverse column permutation)
	// positions, so no permutation pass is needed at the end: assembly row r
	// of the solution lives at c row colPermutation[r].
	var c *discmath.MatrixGF256
	if keepResult {
		c = discmath.NewMatrixGF256(p._L, symSz)
	} else {
		c = arena.newGF256Dirty(p._L, symSz)
	}
	for r := uint32(0); r < nonU; r++ {
		c.RowSet(colPermutation[uSize+r], smallC.GetRow(gaussPerm[r]))
	}

	// Back-substitution through the original upper rows: x_r = D_r ^ the XOR
	// of the other solved columns of row r, its U part (c < r) is already done
	srcIdx := arena.newU32Dirty(int(upperIndex.maxRowLen))
	for r := uint32(0); r < uSize; r++ {
		cols := upperIndex.rowColsFor(r)
		idx := srcIdx[:len(cols)]
		for i, col := range cols {
			idx[i] = colPermutation[col]
		}
		discmath.OctVecAddRowsTo(c.GetRow(colPermutation[r]), dRow(r), c, idx)
	}

	if keepResult {
		arena.release()
		return c, nil, nil
	}
	return c, arena.release, nil
}

// upperSparseIndex is a row index (CSR) of a sparse binary block. For the U
// rows each row is stored as [below-diagonal U cells | E cells], rowMid
// marks the split and the unit diagonal is implicit.
type upperSparseIndex struct {
	rowStarts []uint32
	rowCols   []uint32
	rowMid    []uint32
	maxRowLen uint32
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

// newRowIndex lays out the row starts from the per-row counts; the caller
// scatters the cells with a cursor copy of rowStarts
func newRowIndex(arena *matrixArena, counts []uint32, nnz uint32) upperSparseIndex {
	rows := uint32(len(counts))
	idx := upperSparseIndex{
		rowStarts: arena.newU32Dirty(int(rows + 1)),
		rowCols:   arena.newU32Dirty(int(nnz)),
	}
	offset := uint32(0)
	for r, n := range counts {
		idx.rowStarts[r] = offset
		offset += n
		idx.maxRowLen = max(idx.maxRowLen, n)
	}
	idx.rowStarts[rows] = offset
	return idx
}

func (idx upperSparseIndex) rowColsFor(row uint32) []uint32 {
	return idx.rowCols[idx.rowStarts[row]:idx.rowStarts[row+1]]
}

// rowUFor returns the below-diagonal U columns of U row row, all < row
func (idx upperSparseIndex) rowUFor(row uint32) []uint32 {
	return idx.rowCols[idx.rowStarts[row]:idx.rowMid[row]]
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
//	| U (idx only)      | E (GF2, idx)    |   rows < uSize
//	| gLeft (gIdx only) | smallAUpper bin |   rows >= uSize
//
// The inactivation decoder makes U unit lower-triangular: when it picks a row,
// every still active column of that row is either the row's pivot or gets
// inactivated in the same step, so a U row has no cell right of its diagonal.
// This is checked here, the forward and back substitutions rely on it.
//
// smallAUpper is filled by the caller-provided zeroed view. The permuted upper
// coordinates are compacted in place into entries.rows/cols, which are dead
// after this call: at iteration i the write index nnz <= i, and the cell at i
// was already consumed at the top of the iteration.
func buildPermutedUpperArena(arena *matrixArena, entries upperMatrixEntries, rPerm, cPerm []uint32, rows, cols, uSize uint32, smallAUpper *discmath.MatrixGF256) (*discmath.PlainMatrixGF2, upperSparseIndex, upperSparseIndex) {
	if !entries.valid() {
		panic("raptorq: upper matrix entries overflow")
	}

	lower := rows - uSize
	e := arena.newGF2(uSize, cols-uSize)

	rowCounts := arena.newU32(int(uSize))
	uCounts := arena.newU32(int(uSize))
	gCounts := arena.newU32(int(lower))
	upperRows := entries.rows[:entries.n]
	upperCols := entries.cols[:entries.n]

	nnz := uint32(0)
	rowNNZ := uint32(0)
	for i := uint32(0); i < entries.n; i++ {
		dstRow := rPerm[entries.rows[i]]
		dstCol := cPerm[entries.cols[i]]
		switch {
		case dstRow < uSize && dstCol < uSize:
			if dstCol == dstRow {
				continue // the implicit unit diagonal
			}
			if dstCol > dstRow {
				panic("raptorq: U is not lower triangular")
			}
			uCounts[dstRow]++
			rowCounts[dstRow]++
			rowNNZ++
		case dstRow < uSize:
			e.Set(dstRow, dstCol-uSize)
			rowCounts[dstRow]++
			rowNNZ++
		case dstCol < uSize:
			gCounts[dstRow-uSize]++
		default:
			smallAUpper.Set(dstRow-uSize, dstCol-uSize, 1)
			continue
		}
		upperRows[nnz] = dstRow
		upperCols[nnz] = dstCol
		nnz++
	}

	idx := newRowIndex(arena, rowCounts, rowNNZ)
	gIdx := newRowIndex(arena, gCounts, nnz-rowNNZ)

	// U cells fill each U row from its start, E cells from rowMid on
	idx.rowMid = arena.newU32Dirty(int(uSize))
	uCursor := arena.newU32Dirty(int(uSize))
	eCursor := arena.newU32Dirty(int(uSize))
	gCursor := arena.newU32Dirty(int(lower))
	for r := uint32(0); r < uSize; r++ {
		uCursor[r] = idx.rowStarts[r]
		idx.rowMid[r] = idx.rowStarts[r] + uCounts[r]
		eCursor[r] = idx.rowMid[r]
	}
	copy(gCursor, gIdx.rowStarts[:lower])

	for i := uint32(0); i < nnz; i++ {
		dstRow := upperRows[i]
		dstCol := upperCols[i]

		switch {
		case dstRow >= uSize:
			pos := gCursor[dstRow-uSize]
			gIdx.rowCols[pos] = dstCol
			gCursor[dstRow-uSize] = pos + 1
		case dstCol < uSize:
			pos := uCursor[dstRow]
			idx.rowCols[pos] = dstCol
			uCursor[dstRow] = pos + 1
		default:
			pos := eCursor[dstRow]
			idx.rowCols[pos] = dstCol
			eCursor[dstRow] = pos + 1
		}
	}

	return e, gIdx, idx
}
