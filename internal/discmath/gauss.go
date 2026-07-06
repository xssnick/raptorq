package discmath

import "errors"

var ErrNotSolvable = errors.New("not solvable")

// GaussianElimination reduces a (and applies the same row operations to d)
// using Gauss-Jordan elimination with virtual row swaps through rowPerm.
// No physical row permutation is performed: on return, the solution row r
// lives at d.GetRow(rowPerm[r]).
//
// rowPerm must be a.RowsNum() long, its content is fully overwritten.
//
// The a operations are restricted to columns >= the pivot column: after
// column c is eliminated, every row except its pivot has 0 there, and later
// row operations only scale zeros or add rows that are themselves zero in
// those columns, so the skipped prefix stays untouched either way.
func GaussianElimination(a, d *MatrixGF256, rowPerm []uint32) (*MatrixGF256, error) {
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
			if x == 1 {
				OctVecAdd(targetA, pivotA)
				OctVecAdd(d.GetRow(tr), pivotD)
			} else {
				OctVecMulAdd(targetA, pivotA, x)
				OctVecMulAdd(d.GetRow(tr), pivotD, x)
			}
		}
	}

	return d, nil
}
