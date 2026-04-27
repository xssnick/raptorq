//go:build (!amd64 && !arm64) || purego

package discmath

import "unsafe"

func OctVecAdd(x, y []byte) {
	n := len(x)
	xUint64 := *(*[]uint64)(unsafe.Pointer(&x))
	yUint64 := *(*[]uint64)(unsafe.Pointer(&y))

	for i := 0; i < n/8; i++ {
		xUint64[i] ^= yUint64[i]
	}

	for i := n - n%8; i < n; i++ {
		x[i] ^= y[i]
	}
}

func OctVecMul(vector []byte, multiplier uint8) {
	for i := 0; i < len(vector); i++ {
		vector[i] = OctMul(vector[i], multiplier)
	}
}

func OctVecMulAdd(x, y []byte, multiplier uint8) {
	n := len(x)
	table := _MulPreCalc[multiplier]
	xUint64 := *(*[]uint64)(unsafe.Pointer(&x))
	pos := 0
	for i := 0; i < n/8; i++ {
		var prod uint64
		prod |= uint64(table[y[pos]])
		prod |= uint64(table[y[pos+1]]) << 8
		prod |= uint64(table[y[pos+2]]) << 16
		prod |= uint64(table[y[pos+3]]) << 24
		prod |= uint64(table[y[pos+4]]) << 32
		prod |= uint64(table[y[pos+5]]) << 40
		prod |= uint64(table[y[pos+6]]) << 48
		prod |= uint64(table[y[pos+7]]) << 56

		pos += 8
		xUint64[i] ^= prod
	}

	for i := n - n%8; i < n; i++ {
		x[i] ^= table[y[i]]
	}
}
