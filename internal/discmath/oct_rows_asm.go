//go:build (arm64 || amd64) && !purego

package discmath

import "unsafe"

//go:noescape
func octAddRowsAsm(dst, init, base *byte, stride, n int, idx *uint32, k int)

//go:noescape
func octMulAddRowsAsm(dst, base *byte, stride, n int, idx *uint32, muls *byte, k int)

//go:noescape
func octHdpcStepAsm(acc, v, ua, ub *byte, n int)

// octAddRows sets dst = init ^ rows, a nil init counts as zero and init may be
// dst itself; the rows are already bounds checked (see checkRows)
func octAddRows(dst, init []byte, m *MatrixGF256, idx []uint32) {
	n := len(dst)
	blocks := n &^ 15
	if blocks > 0 {
		octAddRowsAsm(unsafe.SliceData(dst), unsafe.SliceData(init), unsafe.SliceData(m.Data),
			int(m.Cols), blocks, unsafe.SliceData(idx), len(idx))
	}
	if blocks == n {
		return
	}

	cols := int(m.Cols)
	for i := blocks; i < n; i++ {
		var x byte
		if init != nil {
			x = init[i]
		}
		for _, r := range idx {
			x ^= m.Data[int(r)*cols+i]
		}
		dst[i] = x
	}
}

func octMulAddRows(dst []byte, m *MatrixGF256, idx []uint32, muls []byte) {
	n := len(dst)
	blocks := n &^ 15
	if blocks > 0 && len(idx) > 0 {
		octMulAddRowsAsm(unsafe.SliceData(dst), unsafe.SliceData(m.Data), int(m.Cols), blocks,
			unsafe.SliceData(idx), unsafe.SliceData(muls), len(idx))
	}
	if blocks == n {
		return
	}

	cols := int(m.Cols)
	for j, r := range idx {
		// pointer into the read-only global, a value copy would memmove 256B
		table := &_MulPreCalc[muls[j]]
		row := m.Data[int(r)*cols:][:n]
		for i := blocks; i < n; i++ {
			dst[i] ^= table[row[i]]
		}
	}
}

// octHdpcStep: every slice is exactly len(acc) long (see OctHdpcStep)
func octHdpcStep(acc, v, ua, ub []byte) {
	n := len(acc)
	m := n &^ 15
	if m > 0 {
		var vp *byte
		if v != nil {
			vp = unsafe.SliceData(v)
		}
		octHdpcStepAsm(unsafe.SliceData(acc), vp, unsafe.SliceData(ua), unsafe.SliceData(ub), m)
	}
	for i := m; i < n; i++ {
		x := octMul2(acc[i])
		if v != nil {
			x ^= v[i]
		}
		acc[i] = x
		ua[i] ^= x
		ub[i] ^= x
	}
}
