//go:build (!amd64 && !arm64) || purego

package discmath

import "encoding/binary"

func OctVecAdd(x, y []byte) {
	n := len(x)
	y = y[:n] // lifts the bounds checks on y out of the loops

	i := 0
	// XOR is byte-wise, only the two sides have to agree on the word order,
	for ; i+8 <= n; i += 8 {
		a, b := x[i:i+8], y[i:i+8]
		binary.NativeEndian.PutUint64(a, binary.NativeEndian.Uint64(a)^binary.NativeEndian.Uint64(b))
	}

	for ; i < n; i++ {
		x[i] ^= y[i]
	}
}

func OctVecMul(vector []byte, multiplier uint8) {
	// pointer into the read-only global, a value copy would memmove 256B
	table := &_MulPreCalc[multiplier]
	for i, v := range vector {
		vector[i] = table[v]
	}
}

func OctVecMulAdd(x, y []byte, multiplier uint8) {
	n := len(x)
	table := &_MulPreCalc[multiplier]
	y = y[:n]

	i := 0
	// lane k of prod holds table[y[i+k]], so x is read and written
	// little-endian to keep lane k lined up with byte i+k everywhere
	for ; i+8 <= n; i += 8 {
		b := y[i : i+8]
		prod := uint64(table[b[0]]) |
			uint64(table[b[1]])<<8 |
			uint64(table[b[2]])<<16 |
			uint64(table[b[3]])<<24 |
			uint64(table[b[4]])<<32 |
			uint64(table[b[5]])<<40 |
			uint64(table[b[6]])<<48 |
			uint64(table[b[7]])<<56

		a := x[i : i+8]
		binary.LittleEndian.PutUint64(a, binary.LittleEndian.Uint64(a)^prod)
	}

	for ; i < n; i++ {
		x[i] ^= table[y[i]]
	}
}
