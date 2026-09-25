//go:build !arm64 || purego

package discmath

// octHdpcStep: every slice is exactly len(acc) long (see OctHdpcStep)
func octHdpcStep(acc, v, ua, ub []byte) {
	OctVecMul(acc, 2)
	if v != nil {
		OctVecAdd(acc, v)
	}
	OctVecAdd(ua, acc)
	OctVecAdd(ub, acc)
}
