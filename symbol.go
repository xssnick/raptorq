package raptorq

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
