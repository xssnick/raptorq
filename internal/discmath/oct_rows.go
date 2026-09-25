package discmath

// The row kernels combine many rows of one matrix into a destination in a
// single pass: each destination block stays in registers while all sources
// are added, so the destination is loaded and stored once instead of once
// per source row. Sources are given as row indexes of m and checked against
// m.Data before any memory is touched, so a bad index panics instead of
// reading out of bounds. Only the first len(dst) bytes of each row are used.

// OctVecAddRowsTo sets dst = first ^ m[idx[0]] ^ m[idx[1]] ^ ..., a nil first
// counts as zero. dst must not overlap first or any source row.
func OctVecAddRowsTo(dst, first []byte, m *MatrixGF256, idx []uint32) {
	if first != nil {
		first = first[:len(dst)]
	}
	checkRows(m, len(dst), idx)
	octAddRows(dst, first, m, idx)
}

// OctVecAddRows does dst ^= m[idx[0]] ^ m[idx[1]] ^ ...; dst must not overlap
// any source row.
func OctVecAddRows(dst []byte, m *MatrixGF256, idx []uint32) {
	checkRows(m, len(dst), idx)
	octAddRows(dst, dst, m, idx)
}

// OctVecMulAddRows does dst ^= muls[0]*m[idx[0]] ^ muls[1]*m[idx[1]] ^ ...;
// dst must not overlap any source row.
func OctVecMulAddRows(dst []byte, m *MatrixGF256, idx []uint32, muls []byte) {
	muls = muls[:len(idx)]
	checkRows(m, len(dst), idx)
	octMulAddRows(dst, m, idx, muls)
}

// checkRows panics unless the first n bytes of every row idx of m are
// inside m.Data
func checkRows(m *MatrixGF256, n int, idx []uint32) {
	if n == 0 || len(idx) == 0 {
		return
	}
	if n > int(m.Cols) || n > len(m.Data) {
		panic("discmath: row kernel destination is longer than the source rows")
	}
	limit := uint64(len(m.Data)-n) / uint64(m.Cols)
	for _, i := range idx {
		if uint64(i) > limit {
			panic("discmath: row index out of range")
		}
	}
}

// OctHdpcStep does acc = 2*acc ^ v, ua ^= acc, ub ^= acc over len(acc) bytes:
// one step of the HDPC GAMMA chain (alpha = 2) whose result is added to the
// two MT rows of the column. A nil v is a zero row. v, ua and ub must be at
// least len(acc) long, ua and ub must not alias each other or acc.
func OctHdpcStep(acc, v, ua, ub []byte) {
	n := len(acc)
	ua, ub = ua[:n], ub[:n]
	if v != nil {
		v = v[:n]
	}
	octHdpcStep(acc, v, ua, ub)
}

// octMul2 multiplies by alpha = 2 modulo the RaptorQ polynomial 0x11D
func octMul2(x byte) byte {
	return x<<1 ^ byte(int8(x)>>7)&0x1d
}
