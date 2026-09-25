package discmath

import (
	"bytes"
	"math/rand"
	"testing"
)

// guarded returns an n byte slice at offset off of a random buffer with
// random guard bytes around it, and a check that the guards are unchanged
func guarded(rnd *rand.Rand, n, off int) ([]byte, func(t *testing.T)) {
	buf := make([]byte, off+n+32)
	rnd.Read(buf)
	orig := append([]byte(nil), buf...)
	return buf[off : off+n : off+n], func(t *testing.T) {
		t.Helper()
		if !bytes.Equal(buf[:off], orig[:off]) || !bytes.Equal(buf[off+n:], orig[off+n:]) {
			t.Fatalf("write outside of the %d byte slice", n)
		}
	}
}

func TestOctMul2(t *testing.T) {
	for x := 0; x < 256; x++ {
		if got, want := octMul2(byte(x)), OctMul(byte(x), 2); got != want {
			t.Fatalf("octMul2(%d) = %d, want %d", x, got, want)
		}
	}
}

func TestOctHdpcStep(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for n := 0; n <= 300; n++ {
		for _, withV := range []bool{true, false} {
			off := rnd.Intn(16)
			acc, accOK := guarded(rnd, n, off)
			ua, uaOK := guarded(rnd, n, rnd.Intn(16))
			ub, ubOK := guarded(rnd, n, rnd.Intn(16))
			var v []byte
			if withV {
				v = make([]byte, n)
				rnd.Read(v)
			}

			wantAcc := make([]byte, n)
			wantUA := append([]byte(nil), ua...)
			wantUB := append([]byte(nil), ub...)
			for i := range wantAcc {
				x := OctMul(acc[i], 2)
				if v != nil {
					x ^= v[i]
				}
				wantAcc[i] = x
				wantUA[i] ^= x
				wantUB[i] ^= x
			}

			OctHdpcStep(acc, v, ua, ub)
			accOK(t)
			uaOK(t)
			ubOK(t)
			if !bytes.Equal(acc, wantAcc) || !bytes.Equal(ua, wantUA) || !bytes.Equal(ub, wantUB) {
				t.Fatalf("n=%d v=%v: wrong result", n, withV)
			}
		}
	}
}

func TestOctHdpcStepShortRows(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a short ua")
		}
	}()
	OctHdpcStep(make([]byte, 64), nil, make([]byte, 63), make([]byte, 64))
}

func randRows(rnd *rand.Rand, rows, cols int) *MatrixGF256 {
	m := NewMatrixGF256(uint32(rows), uint32(cols))
	rnd.Read(m.Data)
	return m
}

func TestOctVecAddRows(t *testing.T) {
	rnd := rand.New(rand.NewSource(2))
	lengths := []int{0, 1, 15, 16, 17, 100, 127, 128, 129, 250, 255, 256, 300, 768, 1400}
	for n := 0; n <= 64; n++ {
		lengths = append(lengths, n)
	}
	for _, n := range lengths {
		for _, k := range []int{0, 1, 2, 3, 7, 16, 40} {
			m := randRows(rnd, 50, n+rnd.Intn(20))
			idx := make([]uint32, k)
			for j := range idx {
				idx[j] = uint32(rnd.Intn(50)) // duplicates allowed, they cancel
			}

			rowsXor := make([]byte, n)
			for _, r := range idx {
				for i := range rowsXor {
					rowsXor[i] ^= m.GetRow(r)[i]
				}
			}

			for mode := 0; mode < 3; mode++ {
				dst, dstOK := guarded(rnd, n, rnd.Intn(16))
				want := append([]byte(nil), rowsXor...)
				switch mode {
				case 0: // To, nil first
					OctVecAddRowsTo(dst, nil, m, idx)
				case 1: // To, with first
					first := make([]byte, n+rnd.Intn(4))
					rnd.Read(first)
					for i := range want {
						want[i] ^= first[i]
					}
					OctVecAddRowsTo(dst, first, m, idx)
				case 2: // accumulate
					for i := range want {
						want[i] ^= dst[i]
					}
					OctVecAddRows(dst, m, idx)
				}
				dstOK(t)
				if !bytes.Equal(dst, want) {
					t.Fatalf("n=%d k=%d mode=%d: wrong result", n, k, mode)
				}
			}

			muls := make([]byte, k)
			rnd.Read(muls)
			if k > 1 {
				muls[0], muls[1] = 0, 1
			}
			dst, dstOK := guarded(rnd, n, rnd.Intn(16))
			want := append([]byte(nil), dst...)
			for j, r := range idx {
				for i := range want {
					want[i] ^= OctMul(muls[j], m.GetRow(r)[i])
				}
			}
			OctVecMulAddRows(dst, m, idx, muls)
			dstOK(t)
			if !bytes.Equal(dst, want) {
				t.Fatalf("n=%d k=%d: wrong mul-add result", n, k)
			}
		}
	}
}

func TestOctVecAddRowsBounds(t *testing.T) {
	m := NewMatrixGF256(4, 32)
	mustPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatalf("%s: expected a panic", name)
			}
		}()
		fn()
	}
	mustPanic("row past the end", func() { OctVecAddRowsTo(make([]byte, 32), nil, m, []uint32{1, 4}) })
	mustPanic("huge row", func() { OctVecAddRows(make([]byte, 16), m, []uint32{1 << 31}) })
	mustPanic("dst longer than rows", func() { OctVecAddRows(make([]byte, 33), m, []uint32{0}) })
	mustPanic("short first", func() { OctVecAddRowsTo(make([]byte, 32), make([]byte, 31), m, []uint32{0}) })
	mustPanic("short muls", func() { OctVecMulAddRows(make([]byte, 32), m, []uint32{0, 1}, []byte{2}) })
	mustPanic("mul row past the end", func() { OctVecMulAddRows(make([]byte, 32), m, []uint32{7}, []byte{2}) })

	// a shorter destination may read the last row of a truncated view
	view := &MatrixGF256{Rows: 4, Cols: 32, Data: m.Data[:3*32+16]}
	OctVecAddRows(make([]byte, 16), view, []uint32{3})
	mustPanic("truncated view", func() { OctVecAddRows(make([]byte, 17), view, []uint32{3}) })
}
