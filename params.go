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

	actual, _ := paramsCache.LoadOrStore(key, p)
	return actual.(*raptorParams), nil
}

var degreeDistribution = []uint32{
	0, 5243, 529531, 704294, 791675, 844104, 879057, 904023, 922747, 937311, 948962,
	958494, 966438, 973160, 978921, 983914, 988283, 992138, 995565, 998631, 1001391, 1003887,
	1006157, 1008229, 1010129, 1011876, 1013490, 1014983, 1016370, 1017662, 1048576,
}

func (p *raptorParams) getDegree(v uint32) uint32 {
	for i, d := range degreeDistribution {
		if v < d {
			x := p._W - 2
			if x < uint32(i) {
				return x
			}
			return uint32(i)
		}
	}
	panic("should be unreachable")
}

func (p *raptorParams) calcEncodingRow(x uint32) encodingRow {
	ja := 53591 + p._J*997
	if ja%2 == 0 {
		ja++
	}

	bLocal := 10267 * (p._J + 1)
	y := bLocal + x*ja
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

func (p *raptorParams) hdpcMultiply(arena *matrixArena, v *discmath.MatrixGF256) *discmath.MatrixGF256 {
	alpha := discmath.OctExp(1)
	for i := uint32(1); i < v.RowsNum(); i++ {
		v.RowAddMul(i, v.GetRow(i-1), alpha)
	}

	u := arena.newGF256(p._H, v.ColsNum())
	for i := uint32(0); i < p._H; i++ {
		u.RowAddMul(i, v.GetRow(v.RowsNum()-1), discmath.OctExp(i%255))
	}

	for col := uint32(0); col+1 < v.RowsNum(); col++ {
		a := random(col+1, 6, p._H)
		b := (a + random(col+1, 7, p._H-1) + 1) % p._H
		u.RowAdd(a, v.GetRow(col))
		u.RowAdd(b, v.GetRow(col))
	}
	return u
}

func (r *encodingRow) Size() uint32 {
	return r.d + r.d1
}

func (r *encodingRow) encode(aUpper *upperMatrixBuilder, ri uint32, p *raptorParams) {
	aUpper.set(ri+p._S, r.b)

	for j := uint32(1); j < r.d; j++ {
		r.b = (r.b + r.a) % p._W
		aUpper.set(ri+p._S, r.b)
	}

	for r.b1 >= p._P {
		r.b1 = (r.b1 + r.a1) % p._P1
	}

	aUpper.set(ri+p._S, p._W+r.b1)
	for j := uint32(1); j < r.d1; j++ {
		r.b1 = (r.b1 + r.a1) % p._P1
		for r.b1 >= p._P {
			r.b1 = (r.b1 + r.a1) % p._P1
		}
		aUpper.set(ri+p._S, p._W+r.b1)
	}
}

func (r encodingRow) encodeGen(dst []byte, relaxed *discmath.MatrixGF256, p *raptorParams) {
	discmath.OctVecAdd(dst, relaxed.GetRow(r.b))

	for j := uint32(1); j < r.d; j++ {
		r.b = (r.b + r.a) % p._W
		discmath.OctVecAdd(dst, relaxed.GetRow(r.b))
	}

	for r.b1 >= p._P {
		r.b1 = (r.b1 + r.a1) % p._P1
	}

	discmath.OctVecAdd(dst, relaxed.GetRow(p._W+r.b1))
	for j := uint32(1); j < r.d1; j++ {
		r.b1 = (r.b1 + r.a1) % p._P1
		for r.b1 >= p._P {
			r.b1 = (r.b1 + r.a1) % p._P1
		}
		discmath.OctVecAdd(dst, relaxed.GetRow(p._W+r.b1))
	}
}

func (p *raptorParams) genSymbol(relaxed *discmath.MatrixGF256, symbolSz, id uint32) []byte {
	out := make([]byte, symbolSz)
	p.genSymbolInto(out, relaxed, id)
	return out
}

func (p *raptorParams) genSymbolInto(dst []byte, relaxed *discmath.MatrixGF256, id uint32) {
	clear(dst)
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
