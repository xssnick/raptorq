//go:build arm64 && !purego

package discmath

import "unsafe"

//go:noescape
func octHdpcStepAsm(acc, v, ua, ub *byte, n int)

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
