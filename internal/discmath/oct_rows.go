package discmath

import "encoding/binary"

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
	last := uint32(0)
	for _, i := range idx {
		last = max(last, i)
	}
	// uint32 * uint32 cannot overflow uint64
	if uint64(last)*uint64(m.Cols) > uint64(len(m.Data)-n) {
		panic("discmath: row index out of range")
	}
}

// octAddRowsWords sets dst[from:] = init[from:] ^ rows without asm; a nil init
// counts as zero and init may be dst itself. The rows are already bounds
// checked (see checkRows).
//
// The last < 16 bytes are covered by two words that may overlap, the first
// and the last 8 (4, 2) bytes of the tail: both are accumulated over all
// sources from the original init content and stored afterwards, so the
// overlapping bytes get the same value twice. This takes one pass over the
// sources whatever the tail length. XOR is byte-wise, only the loads and the
// stores have to agree on the word order.
func octAddRowsWords(dst, init []byte, m *MatrixGF256, idx []uint32, from int) {
	n := len(dst)
	cols, data := int(m.Cols), m.Data

	i := from
	for ; n-i >= 16; i += 8 {
		octAddRowsWord8(dst, init, data, cols, idx, i)
	}

	switch tail := n - i; {
	case tail == 8:
		octAddRowsWord8(dst, init, data, cols, idx, i)
	case tail > 8:
		j := n - 8
		var x, y uint64
		if init != nil {
			x = binary.NativeEndian.Uint64(init[i:])
			y = binary.NativeEndian.Uint64(init[j:])
		}
		for _, r := range idx {
			row := data[int(r)*cols:][:n]
			x ^= binary.NativeEndian.Uint64(row[i:])
			y ^= binary.NativeEndian.Uint64(row[j:])
		}
		binary.NativeEndian.PutUint64(dst[i:], x)
		binary.NativeEndian.PutUint64(dst[j:], y)
	case tail >= 4:
		// j == i for a 4 byte tail, the second word is then skipped
		j := n - 4
		var x, y uint32
		if init != nil {
			x = binary.NativeEndian.Uint32(init[i:])
			y = binary.NativeEndian.Uint32(init[j:])
		}
		if j == i {
			for _, r := range idx {
				x ^= binary.NativeEndian.Uint32(data[int(r)*cols+i:])
			}
		} else {
			for _, r := range idx {
				row := data[int(r)*cols:][:n]
				x ^= binary.NativeEndian.Uint32(row[i:])
				y ^= binary.NativeEndian.Uint32(row[j:])
			}
			binary.NativeEndian.PutUint32(dst[j:], y)
		}
		binary.NativeEndian.PutUint32(dst[i:], x)
	case tail >= 2:
		// 2 or 3 bytes: a 2 byte word and, for 3, the last byte
		x := uint16(0)
		y := byte(0)
		if init != nil {
			x = binary.NativeEndian.Uint16(init[i:])
			y = init[n-1]
		}
		for _, r := range idx {
			row := data[int(r)*cols:][:n]
			x ^= binary.NativeEndian.Uint16(row[i:])
			y ^= row[n-1]
		}
		binary.NativeEndian.PutUint16(dst[i:], x)
		if tail == 3 {
			dst[n-1] = y
		}
	case tail == 1:
		var x byte
		if init != nil {
			x = init[i]
		}
		for _, r := range idx {
			x ^= data[int(r)*cols+i]
		}
		dst[i] = x
	}
}

// octAddRowsWord8 sets the 8 byte word at dst[i:] to init ^ rows
func octAddRowsWord8(dst, init, data []byte, cols int, idx []uint32, i int) {
	var x uint64
	if init != nil {
		x = binary.NativeEndian.Uint64(init[i:])
	}
	for _, r := range idx {
		x ^= binary.NativeEndian.Uint64(data[int(r)*cols+i:])
	}
	binary.NativeEndian.PutUint64(dst[i:], x)
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
