//go:build arm64 && !purego

package discmath

//go:noescape
func OctVecAdd(x, y []byte)

//go:noescape
func OctVecMul(vector []byte, multiplier uint8)

//go:noescape
func OctVecMulAdd(x, y []byte, multiplier uint8)
