package raptorq

import "testing"

// The solver builds the sparse upper matrix without a duplicate-cell check.
// That is only correct while the generators cannot emit the same cell twice:
//   - LT tuples walk b += a (mod W): distinct while W is prime and d <= W-2;
//   - PI tuples walk over P1, which is prime by construction;
//   - LDPC1 duplicates are filtered locally in solve;
//   - LDPC2 cells are distinct while P >= 2.
//
// This test guards those invariants for every entry of the params table.
func Test_ParamsTableInvariants(t *testing.T) {
	for _, raw := range ParamsTable {
		if !isPrime(raw.W) {
			t.Fatalf("K'=%d: W=%d is not prime", raw.KPadded, raw.W)
		}

		l := raw.KPadded + raw.S + raw.H
		p := l - raw.W
		if p < 2 {
			t.Fatalf("K'=%d: P=%d is less than 2", raw.KPadded, p)
		}
		if p < raw.H {
			t.Fatalf("K'=%d: P=%d is less than H=%d", raw.KPadded, p, raw.H)
		}
	}
}
