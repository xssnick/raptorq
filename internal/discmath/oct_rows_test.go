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
