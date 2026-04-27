//go:build amd64

package discmath

import "unsafe"

var _Mul4bitPreCalc = calcOctMul4bitTable()
var _HasAVX2 = detectAVX2()

func calcOctMul4bitTable() [256][32]uint8 {
	var result [256][32]uint8
	for m := 0; m < 256; m++ {
		for i := 0; i < 16; i++ {
			result[m][i] = _MulPreCalc[m][i]
			result[m][16+i] = _MulPreCalc[m][i<<4]
		}
	}
	return result
}

//go:noescape
func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)

//go:noescape
func xgetbv() (eax, edx uint32)

func detectAVX2() bool {
	_, _, ecx1, _ := cpuid(1, 0)
	const (
		osxsave = 1 << 27
		avx     = 1 << 28
	)
	if ecx1&(osxsave|avx) != osxsave|avx {
		return false
	}

	xcr0, _ := xgetbv()
	if xcr0&0x6 != 0x6 {
		return false
	}

	_, ebx7, _, _ := cpuid(7, 0)
	const avx2 = 1 << 5
	return ebx7&avx2 != 0
}

//go:noescape
func asmSSE2XORBlocks(x, y unsafe.Pointer, blocks int)

//go:noescape
func asmSSSE3MulAdd(x, y unsafe.Pointer, table unsafe.Pointer, blocks int)

//go:noescape
func asmSSSE3Mul(x unsafe.Pointer, table unsafe.Pointer, blocks int)

//go:noescape
func asmAVX2XORBlocks(x, y unsafe.Pointer, blocks int)

func OctVecAdd(x, y []byte) {
	n := len(x)

	i := 0
	if _HasAVX2 {
		blocks := n / 32
		if blocks > 0 {
			asmAVX2XORBlocks(
				unsafe.Pointer(&x[0]),
				unsafe.Pointer(&y[0]),
				blocks,
			)
			i = blocks * 32
		}
	}

	blocks := (n - i) / 16
	if blocks > 0 {
		asmSSE2XORBlocks(
			unsafe.Pointer(&x[i]),
			unsafe.Pointer(&y[i]),
			blocks,
		)
		i += blocks * 16
	}

	// xor rest using 64-bit chunks first
	for ; i+8 <= n; i += 8 {
		*(*uint64)(unsafe.Pointer(&x[i])) ^= *(*uint64)(unsafe.Pointer(&y[i]))
	}
	for ; i < n; i++ {
		x[i] ^= y[i]
	}
}

func OctVecMul(vector []byte, multiplier uint8) {
	n := len(vector)
	if n == 0 {
		return
	}

	table4 := _Mul4bitPreCalc[multiplier]
	blocks := n / 16
	if blocks > 0 {
		asmSSSE3Mul(
			unsafe.Pointer(&vector[0]),
			unsafe.Pointer(&table4[0]),
			blocks,
		)
	}

	table := _MulPreCalc[multiplier]
	for i := blocks * 16; i < n; i++ {
		vector[i] = table[vector[i]]
	}
}

func OctVecMulAdd(x, y []byte, multiplier uint8) {
	n := len(x)
	if n == 0 {
		return
	}
	table := _Mul4bitPreCalc[multiplier]
	blocks := n / 16
	if blocks > 0 {
		asmSSSE3MulAdd(
			unsafe.Pointer(&x[0]),
			unsafe.Pointer(&y[0]),
			unsafe.Pointer(&table[0]),
			blocks,
		)
	}
	full := _MulPreCalc[multiplier]
	for i := blocks * 16; i < n; i++ {
		x[i] ^= full[y[i]]
	}
}
