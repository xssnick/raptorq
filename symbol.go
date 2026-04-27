package raptorq

type symbol struct {
	ID   uint32
	Data []byte
}

type Symbol = symbol

func splitToSymbols(symCount, symSz uint32, data []byte) []symbol {
	symbols := make([]symbol, symCount)
	sym := make([]byte, symSz*symCount)

	for i := uint32(0); i < symCount; i++ {
		offset := i * symSz
		if offset < uint32(len(data)) {
			copy(sym[offset:offset+symSz], data[offset:])
		}
		symbols[i] = symbol{
			ID:   i,
			Data: sym[offset : offset+symSz],
		}
	}

	return symbols
}
