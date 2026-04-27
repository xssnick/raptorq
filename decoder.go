package raptorq

import (
	"errors"
	"fmt"
	"sync"
)

type Decoder struct {
	symbolSz uint32
	dataSz   uint32

	fastNum uint32
	slowNum uint32

	fastSeen    []bool
	fastSymbols []byte
	slowIndex   map[uint32]uint32
	slowIDs     []uint32
	slowSymbols []byte

	pm *raptorParams
}

var symbolSlicePool sync.Pool

func getSymbolSlice(capacity int) []symbol {
	if cached := symbolSlicePool.Get(); cached != nil {
		s := cached.([]symbol)
		if cap(s) >= capacity {
			return s[:0]
		}
	}
	return make([]symbol, 0, capacity)
}

func putSymbolSlice(s []symbol) {
	const maxRetainedSymbols = 64 << 10
	if cap(s) > maxRetainedSymbols {
		return
	}
	clear(s)
	symbolSlicePool.Put(s[:0])
}

func (r *RaptorQ) CreateDecoder(dataSize uint32) (*Decoder, error) {
	param, err := r.calcParams(dataSize)
	if err != nil {
		return nil, fmt.Errorf("failed to calc params: %w", err)
	}

	return &Decoder{
		symbolSz:    r.symbolSz,
		pm:          param,
		dataSz:      dataSize,
		fastSeen:    make([]bool, param._K),
		fastSymbols: make([]byte, param._K*r.symbolSz),
	}, nil
}

func (d *Decoder) AddSymbol(id uint32, data []byte) (bool, error) {
	if uint32(len(data)) != d.symbolSz {
		return false, fmt.Errorf("incorrect symbol size %d, should be %d", len(data), d.symbolSz)
	}

	if id < d.pm._K {
		if d.fastSeen[id] {
			return d.fastNum+d.slowNum >= d.pm._K, nil
		}
		copy(d.fastSymbol(id), data)
		d.fastSeen[id] = true
		d.fastNum++
	} else {
		if d.slowIndex != nil {
			if _, ok := d.slowIndex[id]; ok {
				return d.fastNum+d.slowNum >= d.pm._K, nil
			}
		} else {
			for _, slowID := range d.slowIDs {
				if slowID == id {
					return d.fastNum+d.slowNum >= d.pm._K, nil
				}
			}

			const slowMapThreshold = 16
			if len(d.slowIDs) >= slowMapThreshold {
				d.slowIndex = make(map[uint32]uint32, len(d.slowIDs)+1)
				for i, slowID := range d.slowIDs {
					d.slowIndex[slowID] = uint32(i)
				}
			}
		}

		if d.slowSymbols == nil {
			capSymbols := d.pm._K - d.fastNum + 1
			if capSymbols == 0 {
				capSymbols = 1
			}
			if d.fastNum == 0 && capSymbols > 64 {
				capSymbols = 64
			}
			d.slowIDs = make([]uint32, 0, capSymbols)
			d.slowSymbols = make([]byte, 0, capSymbols*d.symbolSz)
		}
		d.slowSymbols = append(d.slowSymbols, data...)
		d.slowIDs = append(d.slowIDs, id)
		if d.slowIndex != nil {
			d.slowIndex[id] = uint32(len(d.slowIDs) - 1)
		}
		d.slowNum++
	}

	return d.fastNum+d.slowNum >= d.pm._K, nil
}

func (d *Decoder) FastSymbolsNumRequired() uint32 {
	return d.pm._K
}

func (d *Decoder) Decode() (bool, []byte, error) {
	if d.fastNum+d.slowNum < d.pm._K {
		return false, nil, fmt.Errorf("not enough symbols to decode")
	}

	if d.fastNum == d.pm._K {
		out := make([]byte, d.dataSz)
		copy(out, d.fastSymbols)
		return true, out, nil
	}

	// Build system for Solve from known symbols (no payload copy).
	sz := d.pm._K + d.slowNum
	if sz < d.pm._KPadded {
		sz = d.pm._KPadded
	}
	toRelax := getSymbolSlice(int(sz))
	defer func() {
		putSymbolSlice(toRelax)
	}()

	// add known symbols
	for i := uint32(0); i < d.pm._K; i++ {
		if d.fastSeen[i] {
			toRelax = append(toRelax, symbol{ID: i, Data: d.fastSymbol(i)})
		}
	}

	for i, slowID := range d.slowIDs {
		k := slowID
		if k >= d.pm._K {
			// add offset for additional symbols
			k = k + d.pm._KPadded - d.pm._K
		}
		toRelax = append(toRelax, symbol{ID: k, Data: d.slowSymbol(uint32(i) * d.symbolSz)})
	}

	// add padding empty symbols
	for i := uint32(len(toRelax)); i < d.pm._KPadded; i++ {
		toRelax = append(toRelax, symbol{
			ID:   i,
			Data: d.pm.zeroSymbol,
		})
	}

	// we have not all fast symbols, try to recover them from slow
	relaxed, release, err := d.pm.solve(toRelax, false)
	if err != nil {
		if errors.Is(err, errNotEnoughSymbols) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("failed to relax known symbols, err: %w", err)
	}
	defer release()

	out := make([]byte, d.pm._K*d.symbolSz)
	for i := uint32(0); i < d.pm._K; i++ {
		off := i * d.symbolSz
		dst := out[off : off+d.symbolSz]

		if d.fastSeen[i] {
			copy(dst, d.fastSymbol(i))
		} else {
			d.pm.genSymbolInto(dst, relaxed, i)
		}
	}

	return true, out[:d.dataSz], nil
}

func (d *Decoder) fastSymbol(id uint32) []byte {
	offset := id * d.symbolSz
	return d.fastSymbols[offset : offset+d.symbolSz]
}

func (d *Decoder) slowSymbol(offset uint32) []byte {
	return d.slowSymbols[offset : offset+d.symbolSz]
}
