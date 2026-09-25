package discmath

import "errors"

var ErrNotSolvable = errors.New("not solvable")

// GaussianElimination solves a*x = d for the a.ColsNum() unknown rows,
// using virtual row swaps through rowPerm. No physical row permutation is
// performed: on return, the solution row r lives at d.GetRow(rowPerm[r]).
// a is destroyed; the d rows past the solution are left in an unspecified
// state.
//
// It is an LU decomposition: the elimination runs on a alone, below the
// pivots only, keeping each multiplier in the cell it eliminates, and d is
// then updated once per row by a forward and a back substitution with the
// multi-row kernels. The pivot at column k is the first nonzero in rowPerm
// order among rows >= k, whose column k is the same as it would be in
// Gauss-Jordan elimination, so the pivots and the solution are the same.
//
// rowPerm must be a.RowsNum() long, idx and muls at least 2*a.ColsNum()
// long; all three are scratch that is fully overwritten.
func GaussianElimination(a, d *MatrixGF256, rowPerm, idx []uint32, muls []byte) (*MatrixGF256, error) {
	rows, cols := a.Rows, a.Cols

	rowPerm = rowPerm[:rows]
	for i := uint32(0); i < rows; i++ {
		rowPerm[i] = i
	}

	// scale[k] is the inverse of pivot k, 1 when no scaling is needed
	scale := muls[:cols]
	for k := uint32(0); k < cols; k++ {
		nonZero := k
		var pivot uint8
		for nonZero < rows {
			if pivot = a.Get(rowPerm[nonZero], k); pivot != 0 {
				break
			}
			nonZero++
		}
		if nonZero == rows {
			return nil, ErrNotSolvable
		}

		if nonZero != k {
			rowPerm[nonZero], rowPerm[k] = rowPerm[k], rowPerm[nonZero]
		}

		pivotA := a.GetRow(rowPerm[k])[k:]
		scale[k] = 1
		if pivot != 1 {
			scale[k] = OctInverse(pivot)
			OctVecMul(pivotA, scale[k])
		}

		pivotTail := pivotA[1:]
		for t := k + 1; t < rows; t++ {
			targetA := a.GetRow(rowPerm[t])[k:]
			x := targetA[0]
			if x == 0 {
				continue
			}
			// targetA[0] keeps x as the L multiplier of this row operation
			if x == 1 {
				OctVecAdd(targetA[1:], pivotTail)
			} else {
				OctVecMulAdd(targetA[1:], pivotTail, x)
			}
		}
	}

	addIdx, mulIdx, mulVals := idx[:cols], idx[cols:2*cols], muls[cols:2*cols]

	// replay splits the cells of a row span into the rows to add and the
	// rows to multiply-add (at rowPerm positions from+j) and applies them
	replay := func(dRow, span []byte, from uint32) {
		xs, ms, mv := addIdx[:0], mulIdx[:0], mulVals[:0]
		for j, x := range span {
			switch x {
			case 0:
			case 1:
				xs = append(xs, rowPerm[from+uint32(j)])
			default:
				ms = append(ms, rowPerm[from+uint32(j)])
				mv = append(mv, x)
			}
		}
		OctVecAddRows(dRow, d, xs)
		OctVecMulAddRows(dRow, d, ms, mv)
	}

	// forward: d[p_i] = scale_i * (d[p_i] ^ sum_{k<i} L[i][k] * d[p_k]),
	// the row operations of the elimination in the order they happened
	for i := uint32(0); i < cols; i++ {
		pr := rowPerm[i]
		dRow := d.GetRow(pr)
		replay(dRow, a.GetRow(pr)[:i], 0)
		if scale[i] != 1 {
			OctVecMul(dRow, scale[i])
		}
	}

	// back: d[p_i] ^= sum_{j>i} U[i][j] * d[p_j], U has a unit diagonal
	for i := int(cols) - 1; i >= 0; i-- {
		pr := rowPerm[i]
		replay(d.GetRow(pr), a.GetRow(pr)[i+1:], uint32(i+1))
	}

	return d, nil
}
