package raptorq

import (
	"fmt"
	"sync"

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
	_K       uint32
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

	zeroSymbol []byte
}

type paramsCacheKey struct {
	symbolSize uint32
	dataSize   uint32
}

var paramsCache sync.Map

func (r *RaptorQ) calcParams(dataSize uint32) (*raptorParams, error) {
	if r.symbolSz == 0 {
		return nil, fmt.Errorf("symbol size cannot be zero")
	}

	key := paramsCacheKey{
		symbolSize: r.symbolSz,
		dataSize:   dataSize,
	}
	if cached, ok := paramsCache.Load(key); ok {
		return cached.(*raptorParams), nil
	}

	k := (dataSize + r.symbolSz - 1) / r.symbolSz
	raw, err := calcRawParams(k)
	if err != nil {
		return nil, fmt.Errorf("failed to calc params: %w", err)
	}

	p := &raptorParams{
		_K:       k,
		_KPadded: raw.KPadded,
		_J:       raw.J,
		_S:       raw.S,
		_H:       raw.H,
		_W:       raw.W,
		_L:       raw.KPadded + raw.S + raw.H,
		_B:       raw.W - raw.S,

		zeroSymbol: make([]byte, r.symbolSz),
	}

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

	actual, _ := paramsCache.LoadOrStore(key, p)
	return actual.(*raptorParams), nil
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

// hdpcMultiply computes the HDPC rows for v; ab holds the precomputed
// (a, b) random row pairs for every column (see the hdpcAB block in solve),
// they depend only on the column index so they are shared between calls.
func (p *raptorParams) hdpcMultiply(arena *matrixArena, v *discmath.MatrixGF256, ab []uint32) *discmath.MatrixGF256 {
	alpha := discmath.OctExp(1) // == 2, so RowAddMul never hits its 0/1 fast paths
	prev := v.GetRow(0)
	for i := uint32(1); i < v.RowsNum(); i++ {
		cur := v.GetRow(i)
		discmath.OctVecMulAdd(cur, prev, alpha)
		prev = cur
	}

	u := arena.newGF256(p._H, v.ColsNum())
	last := v.GetRow(v.RowsNum() - 1)
	u.RowSet(0, last) // OctExp(0) == 1 and the row is zeroed
	for i := uint32(1); i < p._H; i++ {
		u.RowAddMul(i, last, discmath.OctExp(i%255))
	}

	for col := uint32(0); col+1 < v.RowsNum(); col++ {
		row := v.GetRow(col)
		u.RowAdd(ab[2*col], row)
		u.RowAdd(ab[2*col+1], row)
	}
	return u
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
