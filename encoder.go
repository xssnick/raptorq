package raptorq

import (
	"fmt"

	"github.com/xssnick/raptorq/internal/discmath"
)

type Encoder struct {
	symbolSz uint32
	k        uint32
	relaxed  *discmath.MatrixGF256
	symbols  []symbol
	params   *raptorParams
}

func (r *RaptorQ) CreateEncoder(data []byte) (*Encoder, error) {
	param, k, err := r.calcParams(uint32(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to calc params: %w", err)
	}

	symbols := splitToSymbols(param._KPadded, r.symbolSz, data)

	rx, _, err := param.solve(symbols, true, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to relax symbols: %w", err)
	}

	return &Encoder{
		symbolSz: r.symbolSz,
		k:        k,
		relaxed:  rx,
		symbols:  symbols,
		params:   param,
	}, nil
}

func (e *Encoder) GenSymbol(id uint32) []byte {
	if id < e.k {
		return e.symbols[id].Data
	}

	return e.params.genSymbol(e.relaxed, e.symbolSz, id+e.params._KPadded-e.k)
}

func (e *Encoder) BaseSymbolsNum() uint32 {
	return e.k
}
