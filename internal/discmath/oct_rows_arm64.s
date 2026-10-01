//go:build arm64 && !purego
#include "textflag.h"

// func octHdpcStepAsm(acc, v, ua, ub *byte, n int)
// n is a positive multiple of 16, v may be nil (a zero row).
// acc = 2*acc ^ v; ua ^= acc; ub ^= acc
//
// 2*x = x<<1 ^ (x&0x80 ? 0x1d : 0), polynomial 0x11D. R0-R3 are the load
// pointers, R6-R8 the store pointers of acc, ua and ub (see OctVecAdd).
TEXT ·octHdpcStepAsm(SB), NOSPLIT|NOFRAME, $0-40
    MOVD   acc+0(FP), R0
    MOVD   v+8(FP), R1
    MOVD   ua+16(FP), R2
    MOVD   ub+24(FP), R3
    MOVD   n+32(FP), R4
    MOVD   R0, R6
    MOVD   R2, R7
    MOVD   R3, R8

    VMOVI  $0x80, V30.B16
    VMOVI  $0x1d, V29.B16

    LSR    $6, R4, R5                // R5 = 64B blocks
    AND    $63, R4, R4               // R4 = remaining 16B multiple
    CBZ    R1, zero64

vec64:
    CBZ    R5, vec16
    VLD1.P 64(R0), [V0.B16, V1.B16, V2.B16, V3.B16]
    VLD1.P 64(R1), [V16.B16, V17.B16, V18.B16, V19.B16]
    VCMTST V30.B16, V0.B16, V4.B16   // 0xff where the top bit is set
    VCMTST V30.B16, V1.B16, V5.B16
    VCMTST V30.B16, V2.B16, V6.B16
    VCMTST V30.B16, V3.B16, V7.B16
    VSHL   $1, V0.B16, V0.B16
    VSHL   $1, V1.B16, V1.B16
    VSHL   $1, V2.B16, V2.B16
    VSHL   $1, V3.B16, V3.B16
    VAND   V29.B16, V4.B16, V4.B16
    VAND   V29.B16, V5.B16, V5.B16
    VAND   V29.B16, V6.B16, V6.B16
    VAND   V29.B16, V7.B16, V7.B16
    VEOR   V4.B16, V0.B16, V0.B16    // acc = 2*acc
    VEOR   V5.B16, V1.B16, V1.B16
    VEOR   V6.B16, V2.B16, V2.B16
    VEOR   V7.B16, V3.B16, V3.B16
    VEOR   V16.B16, V0.B16, V0.B16   // acc ^= v
    VEOR   V17.B16, V1.B16, V1.B16
    VEOR   V18.B16, V2.B16, V2.B16
    VEOR   V19.B16, V3.B16, V3.B16
    VST1.P [V0.B16, V1.B16, V2.B16, V3.B16], 64(R6)
    VLD1.P 64(R2), [V20.B16, V21.B16, V22.B16, V23.B16]
    VLD1.P 64(R3), [V24.B16, V25.B16, V26.B16, V27.B16]
    VEOR   V0.B16, V20.B16, V20.B16
    VEOR   V1.B16, V21.B16, V21.B16
    VEOR   V2.B16, V22.B16, V22.B16
    VEOR   V3.B16, V23.B16, V23.B16
    VEOR   V0.B16, V24.B16, V24.B16
    VEOR   V1.B16, V25.B16, V25.B16
    VEOR   V2.B16, V26.B16, V26.B16
    VEOR   V3.B16, V27.B16, V27.B16
    VST1.P [V20.B16, V21.B16, V22.B16, V23.B16], 64(R7)
    VST1.P [V24.B16, V25.B16, V26.B16, V27.B16], 64(R8)
    SUB    $1, R5
    B      vec64

vec16:
    CBZ    R4, done
    VLD1.P 16(R0), [V0.B16]
    VLD1.P 16(R1), [V16.B16]
    VCMTST V30.B16, V0.B16, V4.B16
    VSHL   $1, V0.B16, V0.B16
    VAND   V29.B16, V4.B16, V4.B16
    VEOR   V4.B16, V0.B16, V0.B16
    VEOR   V16.B16, V0.B16, V0.B16
    VST1.P [V0.B16], 16(R6)
    VLD1.P 16(R2), [V20.B16]
    VLD1.P 16(R3), [V24.B16]
    VEOR   V0.B16, V20.B16, V20.B16
    VEOR   V0.B16, V24.B16, V24.B16
    VST1.P [V20.B16], 16(R7)
    VST1.P [V24.B16], 16(R8)
    SUB    $16, R4
    B      vec16

zero64:
    CBZ    R5, zero16
    VLD1.P 64(R0), [V0.B16, V1.B16, V2.B16, V3.B16]
    VCMTST V30.B16, V0.B16, V4.B16
    VCMTST V30.B16, V1.B16, V5.B16
    VCMTST V30.B16, V2.B16, V6.B16
    VCMTST V30.B16, V3.B16, V7.B16
    VSHL   $1, V0.B16, V0.B16
    VSHL   $1, V1.B16, V1.B16
    VSHL   $1, V2.B16, V2.B16
    VSHL   $1, V3.B16, V3.B16
    VAND   V29.B16, V4.B16, V4.B16
    VAND   V29.B16, V5.B16, V5.B16
    VAND   V29.B16, V6.B16, V6.B16
    VAND   V29.B16, V7.B16, V7.B16
    VEOR   V4.B16, V0.B16, V0.B16
    VEOR   V5.B16, V1.B16, V1.B16
    VEOR   V6.B16, V2.B16, V2.B16
    VEOR   V7.B16, V3.B16, V3.B16
    VST1.P [V0.B16, V1.B16, V2.B16, V3.B16], 64(R6)
    VLD1.P 64(R2), [V20.B16, V21.B16, V22.B16, V23.B16]
    VLD1.P 64(R3), [V24.B16, V25.B16, V26.B16, V27.B16]
    VEOR   V0.B16, V20.B16, V20.B16
    VEOR   V1.B16, V21.B16, V21.B16
    VEOR   V2.B16, V22.B16, V22.B16
    VEOR   V3.B16, V23.B16, V23.B16
    VEOR   V0.B16, V24.B16, V24.B16
    VEOR   V1.B16, V25.B16, V25.B16
    VEOR   V2.B16, V26.B16, V26.B16
    VEOR   V3.B16, V27.B16, V27.B16
    VST1.P [V20.B16, V21.B16, V22.B16, V23.B16], 64(R7)
    VST1.P [V24.B16, V25.B16, V26.B16, V27.B16], 64(R8)
    SUB    $1, R5
    B      zero64

zero16:
    CBZ    R4, done
    VLD1.P 16(R0), [V0.B16]
    VCMTST V30.B16, V0.B16, V4.B16
    VSHL   $1, V0.B16, V0.B16
    VAND   V29.B16, V4.B16, V4.B16
    VEOR   V4.B16, V0.B16, V0.B16
    VST1.P [V0.B16], 16(R6)
    VLD1.P 16(R2), [V20.B16]
    VLD1.P 16(R3), [V24.B16]
    VEOR   V0.B16, V20.B16, V20.B16
    VEOR   V0.B16, V24.B16, V24.B16
    VST1.P [V20.B16], 16(R7)
    VST1.P [V24.B16], 16(R8)
    SUB    $16, R4
    B      zero16

done:
    RET

// func octAddRowsAsm(dst, init, base *byte, stride, n int, idx *uint32, k int)
// n is a positive multiple of 16. dst = init ^ row(idx[0]) ^ ... ^ row(idx[k-1])
// where row(i) = base + i*stride; a nil init counts as zero, init may be dst.
// Each 128B (then 16B) block of the result is accumulated in registers over
// all k sources and stored once.
TEXT ·octAddRowsAsm(SB), NOSPLIT|NOFRAME, $0-56
    MOVD   dst+0(FP), R0
    MOVD   init+8(FP), R1
    MOVD   base+16(FP), R2
    MOVD   stride+24(FP), R3
    MOVD   n+32(FP), R4
    MOVD   idx+40(FP), R5
    MOVD   k+48(FP), R6
    MOVD   $0, R7                    // R7 = offset of the current block
    LSR    $7, R4, R8
    LSL    $7, R8, R8                // R8 = end of the 128B blocks

add128:
    CMP    R8, R7
    BEQ    add16
    CBZ    R1, zero128
    ADD    R7, R1, R9
    VLD1.P 64(R9), [V0.B16, V1.B16, V2.B16, V3.B16]
    VLD1   (R9), [V4.B16, V5.B16, V6.B16, V7.B16]
    B      src128
zero128:
    VEOR   V0.B16, V0.B16, V0.B16
    VEOR   V1.B16, V1.B16, V1.B16
    VEOR   V2.B16, V2.B16, V2.B16
    VEOR   V3.B16, V3.B16, V3.B16
    VEOR   V4.B16, V4.B16, V4.B16
    VEOR   V5.B16, V5.B16, V5.B16
    VEOR   V6.B16, V6.B16, V6.B16
    VEOR   V7.B16, V7.B16, V7.B16
src128:
    MOVD   $0, R10
    CBZ    R6, store128
srcloop128:
    MOVWU  (R5)(R10<<2), R11         // row index
    MUL    R3, R11, R11
    ADD    R2, R11, R11
    ADD    R7, R11, R11              // R11 = row start + block offset
    VLD1.P 64(R11), [V16.B16, V17.B16, V18.B16, V19.B16]
    VLD1   (R11), [V20.B16, V21.B16, V22.B16, V23.B16]
    VEOR   V16.B16, V0.B16, V0.B16
    VEOR   V17.B16, V1.B16, V1.B16
    VEOR   V18.B16, V2.B16, V2.B16
    VEOR   V19.B16, V3.B16, V3.B16
    VEOR   V20.B16, V4.B16, V4.B16
    VEOR   V21.B16, V5.B16, V5.B16
    VEOR   V22.B16, V6.B16, V6.B16
    VEOR   V23.B16, V7.B16, V7.B16
    ADD    $1, R10
    CMP    R6, R10
    BNE    srcloop128
store128:
    ADD    R7, R0, R9
    VST1.P [V0.B16, V1.B16, V2.B16, V3.B16], 64(R9)
    VST1   [V4.B16, V5.B16, V6.B16, V7.B16], (R9)
    ADD    $128, R7
    B      add128

add16:
    CMP    R4, R7
    BEQ    addDone
    CBZ    R1, zero16
    ADD    R7, R1, R9
    VLD1   (R9), [V0.B16]
    B      src16
zero16:
    VEOR   V0.B16, V0.B16, V0.B16
src16:
    MOVD   $0, R10
    CBZ    R6, store16
srcloop16:
    MOVWU  (R5)(R10<<2), R11
    MUL    R3, R11, R11
    ADD    R2, R11, R11
    ADD    R7, R11, R11
    VLD1   (R11), [V16.B16]
    VEOR   V16.B16, V0.B16, V0.B16
    ADD    $1, R10
    CMP    R6, R10
    BNE    srcloop16
store16:
    ADD    R7, R0, R9
    VST1   [V0.B16], (R9)
    ADD    $16, R7
    B      add16
addDone:
    RET

// func octMulAddRowsAsm(dst, base *byte, stride, n int, idx *uint32, muls *byte, k int)
// n is a positive multiple of 16, k > 0.
// dst ^= muls[0]*row(idx[0]) ^ ... ^ muls[k-1]*row(idx[k-1]), row(i) = base + i*stride
TEXT ·octMulAddRowsAsm(SB), NOSPLIT|NOFRAME, $0-56
    MOVD   dst+0(FP), R0
    MOVD   base+8(FP), R2
    MOVD   stride+16(FP), R3
    MOVD   n+24(FP), R4
    MOVD   idx+32(FP), R5
    MOVD   muls+40(FP), R12
    MOVD   k+48(FP), R6
    MOVD   $·_OctMulLo(SB), R13
    MOVD   $·_OctMulHi(SB), R14
    VMOVI  $0x0f, V31.B16
    MOVD   $0, R7                    // R7 = offset of the current block
    LSR    $7, R4, R8
    LSL    $7, R8, R8                // R8 = end of the 128B blocks

mul128:
    CMP    R8, R7
    BEQ    mul16
    ADD    R7, R0, R9
    VLD1.P 64(R9), [V0.B16, V1.B16, V2.B16, V3.B16]
    VLD1   (R9), [V4.B16, V5.B16, V6.B16, V7.B16]
    MOVD   $0, R10
msrc128:
    MOVBU  (R12)(R10), R15
    LSL    $4, R15, R15
    ADD    R13, R15, R19
    VLD1   (R19), [V28.B16]          // low nibble table of muls[j]
    ADD    R14, R15, R19
    VLD1   (R19), [V29.B16]          // high nibble table of muls[j]
    MOVWU  (R5)(R10<<2), R11
    MUL    R3, R11, R11
    ADD    R2, R11, R11
    ADD    R7, R11, R11
    VLD1.P 64(R11), [V16.B16, V17.B16, V18.B16, V19.B16]
    VLD1   (R11), [V20.B16, V21.B16, V22.B16, V23.B16]
    VAND   V31.B16, V16.B16, V24.B16
    VUSHR  $4, V16.B16, V16.B16
    VAND   V31.B16, V17.B16, V25.B16
    VUSHR  $4, V17.B16, V17.B16
    VAND   V31.B16, V18.B16, V26.B16
    VUSHR  $4, V18.B16, V18.B16
    VAND   V31.B16, V19.B16, V27.B16
    VUSHR  $4, V19.B16, V19.B16
    VTBL   V24.B16, [V28.B16], V24.B16
    VTBL   V16.B16, [V29.B16], V16.B16
    VTBL   V25.B16, [V28.B16], V25.B16
    VTBL   V17.B16, [V29.B16], V17.B16
    VTBL   V26.B16, [V28.B16], V26.B16
    VTBL   V18.B16, [V29.B16], V18.B16
    VTBL   V27.B16, [V28.B16], V27.B16
    VTBL   V19.B16, [V29.B16], V19.B16
    VEOR   V24.B16, V16.B16, V16.B16
    VEOR   V25.B16, V17.B16, V17.B16
    VEOR   V26.B16, V18.B16, V18.B16
    VEOR   V27.B16, V19.B16, V19.B16
    VEOR   V16.B16, V0.B16, V0.B16
    VEOR   V17.B16, V1.B16, V1.B16
    VEOR   V18.B16, V2.B16, V2.B16
    VEOR   V19.B16, V3.B16, V3.B16
    VAND   V31.B16, V20.B16, V24.B16
    VUSHR  $4, V20.B16, V20.B16
    VAND   V31.B16, V21.B16, V25.B16
    VUSHR  $4, V21.B16, V21.B16
    VAND   V31.B16, V22.B16, V26.B16
    VUSHR  $4, V22.B16, V22.B16
    VAND   V31.B16, V23.B16, V27.B16
    VUSHR  $4, V23.B16, V23.B16
    VTBL   V24.B16, [V28.B16], V24.B16
    VTBL   V20.B16, [V29.B16], V20.B16
    VTBL   V25.B16, [V28.B16], V25.B16
    VTBL   V21.B16, [V29.B16], V21.B16
    VTBL   V26.B16, [V28.B16], V26.B16
    VTBL   V22.B16, [V29.B16], V22.B16
    VTBL   V27.B16, [V28.B16], V27.B16
    VTBL   V23.B16, [V29.B16], V23.B16
    VEOR   V24.B16, V20.B16, V20.B16
    VEOR   V25.B16, V21.B16, V21.B16
    VEOR   V26.B16, V22.B16, V22.B16
    VEOR   V27.B16, V23.B16, V23.B16
    VEOR   V20.B16, V4.B16, V4.B16
    VEOR   V21.B16, V5.B16, V5.B16
    VEOR   V22.B16, V6.B16, V6.B16
    VEOR   V23.B16, V7.B16, V7.B16
    ADD    $1, R10
    CMP    R6, R10
    BNE    msrc128
    ADD    R7, R0, R9
    VST1.P [V0.B16, V1.B16, V2.B16, V3.B16], 64(R9)
    VST1   [V4.B16, V5.B16, V6.B16, V7.B16], (R9)
    ADD    $128, R7
    B      mul128

mul16:
    CMP    R4, R7
    BEQ    mulDone
    ADD    R7, R0, R9
    VLD1   (R9), [V0.B16]
    MOVD   $0, R10
msrc16:
    MOVBU  (R12)(R10), R15
    LSL    $4, R15, R15
    ADD    R13, R15, R19
    VLD1   (R19), [V28.B16]
    ADD    R14, R15, R19
    VLD1   (R19), [V29.B16]
    MOVWU  (R5)(R10<<2), R11
    MUL    R3, R11, R11
    ADD    R2, R11, R11
    ADD    R7, R11, R11
    VLD1   (R11), [V16.B16]
    VAND   V31.B16, V16.B16, V24.B16
    VUSHR  $4, V16.B16, V16.B16
    VTBL   V24.B16, [V28.B16], V24.B16
    VTBL   V16.B16, [V29.B16], V16.B16
    VEOR   V24.B16, V16.B16, V16.B16
    VEOR   V16.B16, V0.B16, V0.B16
    ADD    $1, R10
    CMP    R6, R10
    BNE    msrc16
    VST1   [V0.B16], (R9)
    ADD    $16, R7
    B      mul16
mulDone:
    RET
