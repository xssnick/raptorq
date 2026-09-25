package raptorq

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/golden.txt from the current implementation")

type goldenCase struct {
	size   int
	symSz  uint32
	loss   int    // percent of source symbols dropped before decoding
	extras uint32 // received symbols above K
	large  bool   // skipped in -short mode
}

func goldenCases() []goldenCase {
	var cases []goldenCase
	sizes := []int{1, 17, 100, 1000, 4096, 10000, 65536, 100000, 300001}
	syms := []uint32{1, 7, 16, 20, 64, 100, 127, 128, 129, 255, 256, 768, 1000, 1024, 1400, 1500}
	for i, sz := range sizes {
		for j, ss := range syms {
			if uint32(sz)/ss > 20000 {
				continue
			}
			cases = append(cases, goldenCase{size: sz, symSz: ss, loss: 1 + (7*i+13*j)%60, extras: uint32((i + j) % 3)})
		}
	}
	return append(cases,
		goldenCase{size: 200000, symSz: 20, loss: 50, extras: 400, large: true},
		goldenCase{size: 1 << 20, symSz: 64, loss: 30, extras: 300, large: true},
		goldenCase{size: 1 << 20, symSz: 768, loss: 20, extras: 0, large: true},
		goldenCase{size: 3 << 20, symSz: 1024, loss: 90, extras: 50, large: true},
		goldenCase{size: 500000, symSz: 32, loss: 100, extras: 1000, large: true},
		goldenCase{size: 2 << 20, symSz: 100, loss: 70, extras: 200, large: true},
		goldenCase{size: 51000, symSz: 1, loss: 10, extras: 2, large: true}, // K' >= 50511, H = 16
	)
}

// goldenDigests encodes and decodes one case and returns short digests of all
// generated symbols and of the decode result. Everything is derived from a
// seeded PRNG, so the digests only depend on the RaptorQ implementation.
func goldenDigests(t testing.TB, c goldenCase) (string, string) {
	rnd := rand.New(rand.NewSource(int64(c.size)*7919 + int64(c.symSz)))
	data := make([]byte, c.size)
	rnd.Read(data)

	r := NewRaptorQ(c.symSz)
	enc, err := r.CreateEncoder(data)
	if err != nil {
		t.Fatal(err)
	}
	k := enc.BaseSymbolsNum()

	encHash := sha256.New()
	for id := uint32(0); id < k+50; id++ {
		encHash.Write(enc.GenSymbol(id))
	}
	for id := uint32(1000000); id < 1000000+50*997; id += 997 {
		encHash.Write(enc.GenSymbol(id))
	}

	dec, err := r.CreateDecoder(uint32(c.size))
	if err != nil {
		t.Fatal(err)
	}
	added := uint32(0)
	for id := uint32(0); id < k; id++ {
		if rnd.Intn(100) < c.loss {
			continue
		}
		if _, err = dec.AddSymbol(id, enc.GenSymbol(id)); err != nil {
			t.Fatal(err)
		}
		added++
	}
	for id := k; added < k+c.extras; id += 1 + uint32(rnd.Intn(5)) {
		if _, err = dec.AddSymbol(id, enc.GenSymbol(id)); err != nil {
			t.Fatal(err)
		}
		added++
	}

	decHash := sha256.New()
	ok, got, err := dec.Decode()
	fmt.Fprintf(decHash, "%v %v|", ok, err)
	decHash.Write(got)

	return hex.EncodeToString(encHash.Sum(nil)[:8]), hex.EncodeToString(decHash.Sum(nil)[:8])
}

// Test_GoldenDigests pins the exact encoder output (RFC 6330 compatibility) and
// decoder results for many shapes: small and huge K, symbol sizes that are not
// a multiple of 16, heavy loss, many extra symbols, H = 16. Regenerate with
// go test -run Test_GoldenDigests -update-golden, only for intended changes.
func Test_GoldenDigests(t *testing.T) {
	const path = "testdata/golden.txt"

	var got []string
	for _, c := range goldenCases() {
		if c.large && testing.Short() && !*updateGolden {
			continue
		}
		e, d := goldenDigests(t, c)
		got = append(got, fmt.Sprintf("%d %d %d %d enc=%s dec=%s", c.size, c.symSz, c.loss, c.extras, e, d))
	}

	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	want := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, " enc="); i > 0 {
			want[line[:i]] = line
		}
	}
	if err = sc.Err(); err != nil {
		t.Fatal(err)
	}

	for _, line := range got {
		key := line[:strings.Index(line, " enc=")]
		if w, ok := want[key]; !ok {
			t.Errorf("no golden entry for %q", key)
		} else if w != line {
			t.Errorf("digest mismatch\n got: %s\nwant: %s", line, w)
		}
	}
}

// Test_DecodeOverheadFailures checks that symbols received above K lower the
// decode failure probability, as RFC 6330 promises (about 1e-6 at an overhead
// of 2). The known-zero padding rows used to be replaced by the extra symbols,
// which kept the failure rate at about 0.5% whatever the overhead.
func Test_DecodeOverheadFailures(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for _, c := range []struct {
		size  int
		symSz uint32
	}{{4096, 20}, {4096, 50}, {20000, 50}} {
		data := make([]byte, c.size)
		rnd.Read(data)
		r := NewRaptorQ(c.symSz)
		enc, err := r.CreateEncoder(data)
		if err != nil {
			t.Fatal(err)
		}
		k := enc.BaseSymbolsNum()

		fails := 0
		for trial := 0; trial < 300; trial++ {
			dec, _ := r.CreateDecoder(uint32(c.size))
			added := uint32(0)
			for id := uint32(0); id < k; id++ {
				if rnd.Intn(10) < 3 {
					continue
				}
				dec.AddSymbol(id, enc.GenSymbol(id))
				added++
			}
			for id := k + uint32(rnd.Intn(1000000)); added < k+2; id++ {
				dec.AddSymbol(id, enc.GenSymbol(id))
				added++
			}
			ok, _, err := dec.Decode()
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				fails++
			}
		}
		if fails != 0 {
			t.Errorf("size %d symbol %d (K=%d): %d/300 decodes failed at overhead 2", c.size, c.symSz, k, fails)
		}
	}
}

func Benchmark_CreateEncoder1MB(b *testing.B) {
	data := make([]byte, 1<<20)
	rand.New(rand.NewSource(1)).Read(data)
	r := NewRaptorQ(768)

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := r.CreateEncoder(data); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_GenRepairSymbols1MB(b *testing.B) {
	data := make([]byte, 1<<20)
	rand.New(rand.NewSource(1)).Read(data)
	r := NewRaptorQ(768)
	enc, err := r.CreateEncoder(data)
	if err != nil {
		b.Fatal(err)
	}
	k := enc.BaseSymbolsNum()

	b.ReportAllocs()
	b.SetBytes(int64(768 * k / 5))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for id := k; id < k+k/5; id++ {
			enc.GenSymbol(id)
		}
	}
}

// benchDecodeReuse decodes 80% source + 20% repair symbols through a reused
// Decoder and output buffer.
func benchDecodeReuse(b *testing.B, size int, symSz uint32) {
	data := make([]byte, size)
	rand.New(rand.NewSource(2)).Read(data)
	r := NewRaptorQ(symSz)
	enc, err := r.CreateEncoder(data)
	if err != nil {
		b.Fatal(err)
	}
	k := enc.BaseSymbolsNum()
	fast := k * 80 / 100

	var ids []uint32
	var syms [][]byte
	for id := uint32(0); id < fast; id++ {
		ids = append(ids, id)
		syms = append(syms, enc.GenSymbol(id))
	}
	for id := k; id < k+(k-fast); id++ {
		ids = append(ids, id)
		syms = append(syms, enc.GenSymbol(id))
	}

	dec, err := r.CreateDecoder(uint32(size))
	if err != nil {
		b.Fatal(err)
	}
	dst := make([]byte, size)
	decode := func() {
		dec.Reset()
		for i, id := range ids {
			if _, err := dec.AddSymbol(id, syms[i]); err != nil {
				b.Fatal(err)
			}
		}
		if ok, err := dec.DecodeInto(dst); !ok || err != nil {
			b.Fatal("decode failed", ok, err)
		}
	}

	decode()
	if string(dst) != string(data) {
		b.Fatal("decoded data mismatch")
	}

	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decode()
	}
}

// symbol size that is not a multiple of 16 (MTU-derived)
func Benchmark_Decode80Sym1400Reuse(b *testing.B) {
	benchDecodeReuse(b, 1<<20, 1400)
}

func Benchmark_Decode80Reuse16MB(b *testing.B) {
	benchDecodeReuse(b, 16<<20, 1024)
}
