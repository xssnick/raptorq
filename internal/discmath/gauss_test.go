package discmath

import (
	"bytes"
	"math/rand"
	"testing"
)

// gaussJordan is the previous Gauss-Jordan implementation of
// GaussianElimination, kept as the reference: the LU form must pick the same
// pivots and produce the same solution rows.
func gaussJordan(a, d *MatrixGF256, rowPerm []uint32) (*MatrixGF256, error) {
	rows := a.RowsNum()

	rowPerm = rowPerm[:rows]
	for i := uint32(0); i < rows; i++ {
		rowPerm[i] = i
	}

	for row := uint32(0); row < a.ColsNum(); row++ {
		nonZero := row
		var pivot uint8
		for nonZero < rows {
			if pivot = a.Get(rowPerm[nonZero], row); pivot != 0 {
				break
			}
			nonZero++
		}
		if nonZero == rows {
			return nil, ErrNotSolvable
		}

		if nonZero != row {
			rowPerm[nonZero], rowPerm[row] = rowPerm[row], rowPerm[nonZero]
		}

		pr := rowPerm[row]
		pivotA := a.GetRow(pr)[row:]
		pivotD := d.GetRow(pr)

		if pivot != 1 {
			mul := OctInverse(pivot)
			OctVecMul(pivotA, mul)
			OctVecMul(pivotD, mul)
		}

		for zeroRow := uint32(0); zeroRow < rows; zeroRow++ {
			if zeroRow == row {
				continue
			}
			tr := rowPerm[zeroRow]
			targetA := a.GetRow(tr)[row:]
			x := targetA[0]
			if x == 0 {
				continue
			}
			OctVecMulAdd(targetA, pivotA, x)
			OctVecMulAdd(d.GetRow(tr), pivotD, x)
		}
	}

	return d, nil
}

func TestGaussianEliminationMatchesGaussJordan(t *testing.T) {
	rnd := rand.New(rand.NewSource(11))
	solved := 0
	for iter := 0; iter < 2000; iter++ {
		cols := uint32(1 + rnd.Intn(70))
		rows := cols + uint32(rnd.Intn(6))
		width := uint32(1 + rnd.Intn(300))

		// mostly binary cells like the RaptorQ system, some GF(256) rows
		density := 2 + rnd.Intn(6)
		a := NewMatrixGF256(rows, cols)
		for r := uint32(0); r < rows; r++ {
			gf := rnd.Intn(5) == 0
			for c := uint32(0); c < cols; c++ {
				if rnd.Intn(density) != 0 {
					continue
				}
				v := byte(1)
				if gf {
					v = byte(rnd.Intn(256))
				}
				a.Set(r, c, v)
			}
		}
		d := NewMatrixGF256(rows, width)
		rnd.Read(d.Data)

		a2 := &MatrixGF256{Rows: rows, Cols: cols, Data: append([]byte(nil), a.Data...)}
		d2 := &MatrixGF256{Rows: rows, Cols: width, Data: append([]byte(nil), d.Data...)}

		p1 := make([]uint32, rows)
		p2 := make([]uint32, rows)
		r1, err1 := gaussJordan(a, d, p1)
		r2, err2 := GaussianElimination(a2, d2, p2, make([]uint32, 2*cols), make([]byte, 2*cols))
		if (err1 == nil) != (err2 == nil) {
			t.Fatalf("iter %d: error mismatch %v vs %v", iter, err1, err2)
		}
		if err1 != nil {
			continue
		}
		solved++
		for r := uint32(0); r < cols; r++ {
			if p1[r] != p2[r] {
				t.Fatalf("iter %d: pivot %d differs", iter, r)
			}
			if !bytes.Equal(r1.GetRow(p1[r]), r2.GetRow(p2[r])) {
				t.Fatalf("iter %d: solution row %d differs", iter, r)
			}
		}
	}
	if solved < 200 {
		t.Fatalf("only %d of the random systems were solvable", solved)
	}
}
