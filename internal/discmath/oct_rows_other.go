//go:build (!arm64 && !amd64) || purego

package discmath

// octAddRows sets dst = init ^ rows, a nil init counts as zero and init may be
// dst itself; the rows are already bounds checked (see checkRows)
func octAddRows(dst, init []byte, m *MatrixGF256, idx []uint32) {
	n := len(dst)
	if n == 0 {
		return
	}
	if n <= 32 {
		// a few words: cheaper than a copy and one OctVecAdd call per source
		octAddRowsWords(dst, init, m, idx, 0)
		return
	}
	cols := int(m.Cols)
	switch {
	case init == nil && len(idx) > 0:
		copy(dst, m.Data[int(idx[0])*cols:][:n])
		idx = idx[1:]
	case init == nil:
		clear(dst)
	case &init[0] != &dst[0]:
		copy(dst, init)
	}
	for _, i := range idx {
		OctVecAdd(dst, m.Data[int(i)*cols:][:n])
	}
}

func octMulAddRows(dst []byte, m *MatrixGF256, idx []uint32, muls []byte) {
	n := len(dst)
	cols := int(m.Cols)
	for j, i := range idx {
		OctVecMulAdd(dst, m.Data[int(i)*cols:][:n], muls[j])
	}
}

// octHdpcStep: every slice is exactly len(acc) long (see OctHdpcStep)
func octHdpcStep(acc, v, ua, ub []byte) {
	OctVecMul(acc, 2)
	if v != nil {
		OctVecAdd(acc, v)
	}
	OctVecAdd(ua, acc)
	OctVecAdd(ub, acc)
}
