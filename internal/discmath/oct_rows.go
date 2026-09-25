package discmath

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
