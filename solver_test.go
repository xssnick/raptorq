package raptorq

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"
)

func Test_Encode(t *testing.T) {
	str := "hello world bro! keke meme 881"

	r := NewRaptorQ(20)
	enc, err := r.CreateEncoder([]byte(str))
	if err != nil {
		panic(err)
	}

	should, _ := hex.DecodeString("05e6ddeb1f820e0a0f318b23128d889623663e66")
	sx := enc.GenSymbol(68238283)

	if !bytes.Equal(sx, should) {
		t.Fatal("encoded not eq, got", hex.EncodeToString(sx))
	}
}

func Test_EncodeDecode(t *testing.T) {
	str := []byte("hello world bro! keke meme 881")

	r := NewRaptorQ(20)
	enc, err := r.CreateEncoder(str)
	if err != nil {
		panic(err)
	}

	dec, err := r.CreateDecoder(uint32(len(str)))
	if err != nil {
		panic(err)
	}

	for i := uint32(0); i < 2; i++ {
		sx := enc.GenSymbol(i + 10000)

		_, err := dec.AddSymbol(i+10000, sx)
		if err != nil {
			t.Fatal("add symbol err", err)
		}
	}

	_, data, err := dec.Decode()
	if err != nil {
		t.Fatal("decode err", err)
	}

	if !bytes.Equal(data, str) {
		t.Fatal("initial data not eq decoded")
	}
}

func Test_EncodeDecodeBig(t *testing.T) {
	str := make([]byte, 4<<20) // 4mb
	_, _ = rand.Read(str)

	r := NewRaptorQ(768)
	enc, err := r.CreateEncoder(str)
	if err != nil {
		panic(err)
	}

	dec, err := r.CreateDecoder(uint32(len(str)))
	if err != nil {
		panic(err)
	}

	// add half fast symbols
	for i := uint32(0); i < enc.BaseSymbolsNum()-1; i++ {
		sx := enc.GenSymbol(i)

		_, err := dec.AddSymbol(i, sx)
		if err != nil {
			t.Fatal("add symbol err", err)
		}
	}

	// add half slow symbols
	for i := uint32(0); i < 1; i++ {
		sx := enc.GenSymbol(i + enc.BaseSymbolsNum())

		_, err := dec.AddSymbol(i+enc.BaseSymbolsNum(), sx)
		if err != nil {
			t.Fatal("add symbol err", err)
		}
	}

	tm := time.Now()
	ok, data, err := dec.Decode()
	if err != nil {
		t.Fatal("decode err", err)
	}

	if !ok {
		t.Fatal("not decoded")
	}

	println(time.Since(tm).String())

	if !bytes.Equal(data, str) {
		t.Fatal("initial data not eq decrypted")
	}
}

func Test_EncodeDecodeFuzz(t *testing.T) {
	for n := 0; n < 1000; n++ {
		str := make([]byte, 4096)
		_, _ = rand.Read(str)

		buf := make([]byte, 4)
		if _, err := rand.Read(buf); err != nil {
			panic(err)
		}
		rnd := binary.LittleEndian.Uint32(buf)

		symSz := (1 + (rnd % 10)) * 10
		r := NewRaptorQ(symSz)
		enc, err := r.CreateEncoder(str)
		if err != nil {
			panic(err)
		}

		dec, err := r.CreateDecoder(uint32(len(str)))
		if err != nil {
			panic(err)
		}

		_, err = dec.AddSymbol(2, enc.GenSymbol(2))
		if err != nil {
			t.Fatal("add 2 symbol err", err)
		}

		for i := uint32(0); i < enc.params._K; i++ {
			sx := enc.GenSymbol(i + 10000)

			_, err := dec.AddSymbol(i+10000, sx)
			if err != nil {
				t.Fatal("add symbol err", err)
			}
		}

		_, data, err := dec.Decode()
		if err != nil {
			t.Fatal("decode err", err)
		}

		if !bytes.Equal(data, str) {
			t.Fatal("initial data not eq decrypted")
		}
	}
}

// Benchmark_EncodeDecodeFuzz-10    	                        5732	    182353 ns/op	  364865 B/op	     203 allocs/op
func Benchmark_EncodeDecodeFuzz(b *testing.B) {
	str := make([]byte, 4096)
	rand.Read(str)

	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		var symSz uint32 = 768
		r := NewRaptorQ(symSz)
		enc, err := r.CreateEncoder(str)
		if err != nil {
			panic(err)
		}

		dec, err := r.CreateDecoder(uint32(len(str)))
		if err != nil {
			b.Fatal("create decoder err", err)
		}

		_, err = dec.AddSymbol(2, enc.GenSymbol(2))
		if err != nil {
			b.Fatal("add 2 symbol err", err)
		}

		for i := uint32(0); i < enc.params._K; i++ {
			sx := enc.GenSymbol(i + 10000)

			_, err := dec.AddSymbol(i+10000, sx)
			if err != nil {
				b.Fatal("add symbol err", err)
			}
		}

		_, _, err = dec.Decode()
		if err != nil {
			b.Fatal("decode err", err)
		}
	}
}

func Benchmark_Decode80PercentFastRecovery(b *testing.B) {
	str := make([]byte, 1<<20)
	rand.Read(str)

	const symSz uint32 = 768
	r := NewRaptorQ(symSz)
	enc, err := r.CreateEncoder(str)
	if err != nil {
		b.Fatal("create encoder err", err)
	}

	k := enc.params._K
	fastNum := (k*80 + 99) / 100
	if fastNum >= k {
		fastNum = k - 1
	}
	recoverNum := k - fastNum

	fastSymbols := make([][]byte, fastNum)
	for i := uint32(0); i < fastNum; i++ {
		fastSymbols[i] = enc.GenSymbol(i)
	}

	repairSymbols := make([][]byte, recoverNum)
	for i := uint32(0); i < recoverNum; i++ {
		repairSymbols[i] = enc.GenSymbol(k + i)
	}

	decode := func() ([]byte, error) {
		dec, err := r.CreateDecoder(uint32(len(str)))
		if err != nil {
			return nil, err
		}

		for i := uint32(0); i < fastNum; i++ {
			if _, err = dec.AddSymbol(i, fastSymbols[i]); err != nil {
				return nil, err
			}
		}
		for i := uint32(0); i < recoverNum; i++ {
			if _, err = dec.AddSymbol(k+i, repairSymbols[i]); err != nil {
				return nil, err
			}
		}

		ok, data, err := dec.Decode()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errNotEnoughSymbols
		}
		return data, nil
	}

	data, err := decode()
	if err != nil {
		b.Fatal("decode err", err)
	}
	if !bytes.Equal(data, str) {
		b.Fatal("initial data not eq decoded")
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(str)))
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		if _, err = decode(); err != nil {
			b.Fatal("decode err", err)
		}
	}
}
