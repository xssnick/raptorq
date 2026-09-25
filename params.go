package raptorq

import (
	"fmt"

	"github.com/xssnick/raptorq/internal/discmath"
)

type encodingRow struct {
	d  uint32 // [1,30] LT degree
	a  uint32 // [0,W)
	b  uint32 // [0,W)
	d1 uint32 // [2,3]  PI degree
	a1 uint32 // [0,P1)
	b1 uint32 // [0,P1)
}

type raptorParams struct {
	_KPadded uint32
	_J       uint32
	_S       uint32
	_H       uint32
	_W       uint32
	_L       uint32
	_P       uint32
	_P1      uint32
	_U       uint32
	_B       uint32

	// J-derived LT tuple constants, see calcEncodingRow
	_JA     uint32
	_BLocal uint32
}

// Every field of raptorParams derives from the ParamsTable row that K selects,
// so there are only len(ParamsTable) distinct values and all of them are built
// once here. K itself is the only K-dependent value and lives on the Encoder
// and the Decoder instead, which keeps these shared, immutable and pointer
// free, so they never take part in a GC mark.
var paramsByRow = buildParamsByRow()

func buildParamsByRow() []raptorParams {
	all := make([]raptorParams, len(ParamsTable))
	for i := range ParamsTable {
		raw := &ParamsTable[i]
		p := &all[i]

		p._KPadded = raw.KPadded
		p._J = raw.J
		p._S = raw.S
		p._H = raw.H
		p._W = raw.W
		p._L = raw.KPadded + raw.S + raw.H
		p._B = raw.W - raw.S

		p._P = p._L - p._W
		p._U = p._P - p._H
		p._P1 = p._P + 1

		for !isPrime(p._P1) {
			p._P1++
		}

		p._JA = 53591 + p._J*997
		if p._JA%2 == 0 {
			p._JA++
		}
		p._BLocal = 10267 * (p._J + 1)
	}
	return all
}

// calcParams returns the shared params for dataSize together with K, the number
// of source symbols. The returned params are immutable and shared by every
// caller, so anything sized by the symbol size belongs to the caller.
func (r *RaptorQ) calcParams(dataSize uint32) (*raptorParams, uint32, error) {
	if r.symbolSz == 0 {
		return nil, 0, fmt.Errorf("symbol size cannot be zero")
	}

	k := (dataSize + r.symbolSz - 1) / r.symbolSz
	i := rawParamsIndex(k)
	if i < 0 {
		return nil, 0, fmt.Errorf("failed to calc params: %w", errKTooBig)
	}

	return &paramsByRow[i], k, nil
}

var degreeDistribution = [...]uint32{
	0, 5243, 529531, 704294, 791675, 844104, 879057, 904023, 922747, 937311, 948962,
	958494, 966438, 973160, 978921, 983914, 988283, 992138, 995565, 998631, 1001391, 1003887,
	1006157, 1008229, 1010129, 1011876, 1013490, 1014983, 1016370, 1017662, 1048576,
}

func (p *raptorParams) getDegree(v uint32) uint32 {
	// v < 1<<20 == the last table entry, and entry 0 is 0, so the
	// scan always terminates at some i >= 1
	for i := 1; ; i++ {
		if v < degreeDistribution[i] {
			x := p._W - 2
			if x < uint32(i) {
				return x
			}
			return uint32(i)
		}
	}
}

func (p *raptorParams) calcEncodingRow(x uint32) encodingRow {
	y := p._BLocal + x*p._JA
	v := random(y, 0, 1<<20)
	d := p.getDegree(v)
	a := 1 + random(y, 1, p._W-1)
	b := random(y, 2, p._W)

	var d1 uint32
	if d < 4 {
		d1 = 2 + random(x, 3, 2)
	} else {
		d1 = 2
	}

	a1 := 1 + random(x, 4, p._P1-1)
	b1 := random(x, 5, p._P1)

	return encodingRow{
		d:  d,
		a:  a,
		b:  b,
		d1: d1,
		a1: a1,
		b1: b1,
	}
}

// hdpcStream overwrites u (H rows) with HDPC * v, where v has K'+S rows and
// row col of v is rowAt(col), nil for a zero row. ab holds the precomputed
// (a, b) MT row pairs of every column but the last (see the hdpcAB block in
// solve), acc is a len(u row) scratch.
//
// HDPC = MT * GAMMA with GAMMA[i][j] = alpha^(i-j) for i >= j, so
// (GAMMA v)_col = alpha*(GAMMA v)_{col-1} ^ v_col: the chain is kept in the
// single accumulator row and each v row is read once, v is never built.
func (p *raptorParams) hdpcStream(u *discmath.MatrixGF256, acc []byte, ab []uint32, rowAt func(col uint32) []byte) {
	clear(u.Data)
	clear(acc)

	last := p._KPadded + p._S - 1
	for col := uint32(0); col < last; col++ {
		discmath.OctHdpcStep(acc, rowAt(col), u.GetRow(ab[2*col]), u.GetRow(ab[2*col+1]))
	}

	// the last MT column is alpha^i in row i instead of an (a, b) pair
	discmath.OctVecMul(acc, discmath.OctExp(1))
	if v := rowAt(last); v != nil {
		discmath.OctVecAdd(acc, v)
	}
	u.RowAdd(0, acc) // OctExp(0) == 1
	for i := uint32(1); i < p._H; i++ {
		u.RowAddMul(i, acc, discmath.OctExp(i%255))
	}
}

func (r *encodingRow) Size() uint32 {
	return r.d + r.d1
}

// b < W and a < W, so b+a < 2W and a conditional subtract equals % W;
// the same holds for the b1/a1 walk over P1
func (r *encodingRow) encode(aUpper *upperMatrixBuilder, ri uint32, p *raptorParams) {
	w, p1, pp := p._W, p._P1, p._P
	row := ri + p._S

	b := r.b
	aUpper.set(row, b)
	for j := uint32(1); j < r.d; j++ {
		b += r.a
		if b >= w {
			b -= w
		}
		aUpper.set(row, b)
	}

	b1 := r.b1
	for b1 >= pp {
		b1 += r.a1
		if b1 >= p1 {
			b1 -= p1
		}
	}

	aUpper.set(row, w+b1)
	for j := uint32(1); j < r.d1; j++ {
		b1 += r.a1
		if b1 >= p1 {
			b1 -= p1
		}
		for b1 >= pp {
			b1 += r.a1
			if b1 >= p1 {
				b1 -= p1
			}
		}
		aUpper.set(row, w+b1)
	}
}

// encodeGen overwrites dst with the combination of relaxed rows, dst content is ignored
func (r encodingRow) encodeGen(dst []byte, relaxed *discmath.MatrixGF256, p *raptorParams) {
	w, p1, pp := p._W, p._P1, p._P

	b := r.b
	copy(dst, relaxed.GetRow(b))
	for j := uint32(1); j < r.d; j++ {
		b += r.a
		if b >= w {
			b -= w
		}
		discmath.OctVecAdd(dst, relaxed.GetRow(b))
	}

	b1 := r.b1
	for b1 >= pp {
		b1 += r.a1
		if b1 >= p1 {
			b1 -= p1
		}
	}

	discmath.OctVecAdd(dst, relaxed.GetRow(w+b1))
	for j := uint32(1); j < r.d1; j++ {
		b1 += r.a1
		if b1 >= p1 {
			b1 -= p1
		}
		for b1 >= pp {
			b1 += r.a1
			if b1 >= p1 {
				b1 -= p1
			}
		}
		discmath.OctVecAdd(dst, relaxed.GetRow(w+b1))
	}
}

func (p *raptorParams) genSymbol(relaxed *discmath.MatrixGF256, symbolSz, id uint32) []byte {
	out := make([]byte, symbolSz)
	p.genSymbolInto(out, relaxed, id)
	return out
}

func (p *raptorParams) genSymbolInto(dst []byte, relaxed *discmath.MatrixGF256, id uint32) {
	row := p.calcEncodingRow(id)
	row.encodeGen(dst, relaxed, p)
}

func isPrime(n uint32) bool {
	if n <= 3 {
		return true
	}
	if n%2 == 0 || n%3 == 0 {
		return false
	}

	i := uint32(5)
	w := uint32(2)
	for i*i <= n {
		if n%i == 0 {
			return false
		}
		i += w
		w = 6 - w
	}
	return true
}
