package raptorq

import "sync/atomic"

// zeroPad backs the all-zero padding symbols the decoder feeds to solve. solve
// only ever reads symbol payloads out (see createDPermuted), so a single
// read-only buffer can back every padding entry of every decoder. It grows to
// the largest symbol size in use and never further, so unlike a cache keyed by
// data size it is bounded by the symbol size alone.
var zeroPad atomic.Pointer[[]byte]

func zeroSymbol(size uint32) []byte {
	if cur := zeroPad.Load(); cur != nil && uint32(len(*cur)) >= size {
		return (*cur)[:size]
	}

	grown := make([]byte, size)
	for {
		cur := zeroPad.Load()
		if cur != nil && uint32(len(*cur)) >= size {
			return (*cur)[:size]
		}
		if zeroPad.CompareAndSwap(cur, &grown) {
			return grown
		}
	}
}

type symbol struct {
	ID   uint32
	Data []byte
}

type Symbol = symbol

func splitToSymbols(symCount, symSz uint32, data []byte) []symbol {
	symbols := make([]symbol, symCount)
	sym := make([]byte, symSz*symCount)
	copy(sym, data) // the tail past len(data) stays zero padding

	for i := uint32(0); i < symCount; i++ {
		offset := i * symSz
		symbols[i] = symbol{
			ID:   i,
			Data: sym[offset : offset+symSz],
		}
	}

	return symbols
}
