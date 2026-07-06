//go:build arm64 && !purego
#include "textflag.h"

// func OctVecAdd(x, y []byte)
// x: {x_base, x_len, x_cap}, y: {y_base, y_len, y_cap}
//
// R0 is the x store pointer, R4 the x load pointer: keeping them separate
// breaks the serial address dependency of load-after-writeback, both advance
// by the same amounts so they always point at the same block.
TEXT ·OctVecAdd(SB), NOSPLIT|NOFRAME, $0-48
    MOVD   x_base+0(FP), R0
    MOVD   y_base+24(FP), R1
    MOVD   x_len+8(FP), R2
    MOVD   R0, R4

    LSR    $7, R2, R3
    CBZ    R3, after128
loop128:
    VLD1.P 64(R4), [V0.B16, V1.B16, V2.B16, V3.B16]
    VLD1.P 64(R1), [V16.B16, V17.B16, V18.B16, V19.B16]
    VLD1.P 64(R4), [V4.B16, V5.B16, V6.B16, V7.B16]
    VLD1.P 64(R1), [V20.B16, V21.B16, V22.B16, V23.B16]
    VEOR   V16.B16, V0.B16, V0.B16
    VEOR   V17.B16, V1.B16, V1.B16
    VEOR   V18.B16, V2.B16, V2.B16
    VEOR   V19.B16, V3.B16, V3.B16
    VEOR   V20.B16, V4.B16, V4.B16
    VEOR   V21.B16, V5.B16, V5.B16
    VEOR   V22.B16, V6.B16, V6.B16
    VEOR   V23.B16, V7.B16, V7.B16
    VST1.P [V0.B16, V1.B16, V2.B16, V3.B16], 64(R0)
    VST1.P [V4.B16, V5.B16, V6.B16, V7.B16], 64(R0)

    SUBS   $1, R3
    BNE    loop128
after128:
    ANDS   $127, R2, R2
    BEQ    done
    LSR    $5, R2, R3
    CBZ    R3, after32
loop32:
    VLD1.P 32(R4), [V0.B16, V1.B16]
    VLD1.P 32(R1), [V2.B16, V3.B16]
    VEOR   V2.B16, V0.B16, V0.B16
    VEOR   V3.B16, V1.B16, V1.B16
    VST1.P [V0.B16, V1.B16], 32(R0)

    SUBS   $1, R3
    BNE    loop32
after32:
    ANDS   $31, R2, R2
    BEQ    done
    TBZ    $4, R2, tail
    VLD1.P 16(R4), [V0.B16]
    VLD1.P 16(R1), [V1.B16]
    VEOR   V1.B16, V0.B16, V0.B16
    VST1.P [V0.B16], 16(R0)
    AND    $15, R2, R2
    CBZ    R2, done

tail:
    TBZ    $3, R2, chk4
    MOVD   0(R0), R3
    MOVD   0(R1), R4
    EOR    R4, R3, R3
    MOVD   R3, 0(R0)
    ADD    $8, R0
    ADD    $8, R1
chk4:
    TBZ    $2, R2, chk2
    MOVW   0(R0), R3
    MOVW   0(R1), R4
    EORW   R4, R3, R3
    MOVW   R3, 0(R0)
    ADD    $4, R0
    ADD    $4, R1
chk2:
    TBZ    $1, R2, chk1
    MOVHU  0(R0), R3
    MOVHU  0(R1), R4
    EOR    R4, R3, R3
    MOVH   R3, 0(R0)
    ADD    $2, R0
    ADD    $2, R1
chk1:
    TBZ    $0, R2, done
    MOVBU  0(R0), R3
    MOVBU  0(R1), R4
    EOR    R4, R3, R3
    MOVB   R3, 0(R0)
done:
    RET

// func OctVecMul(vector []byte, multiplier uint8)
TEXT ·OctVecMul(SB), NOSPLIT|NOFRAME, $0-25
    MOVD   vector+0(FP), R0          // R0 = &vector[0]
    MOVD   vector_len+8(FP), R1      // R1 = len(vector)
    MOVBU  multiplier+24(FP), R2     // R2 = multiplier

    CMP    $1, R2                    // multiplier 1 is the identity map
    BEQ    done

    MOVD   $·_OctMulLo(SB), R3
    MOVD   $·_OctMulHi(SB), R4
    LSL    $4, R2, R5
    ADD    R5, R3, R3                // R3 = &OctMulLo[multiplier]
    ADD    R5, R4, R4                // R4 = &OctMulHi[multiplier]

    VLD1   (R3), [V2.B16]            // V2 = rowLo
    VLD1   (R4), [V3.B16]            // V3 = rowHi

    VMOVI  $0x0f, V1.B16             // V1 = 0x0F mask

    LSR     $4, R1, R5
    CBZ     R5, after16
loop16:
    VLD1    (R0), [V0.B16]           // V0 = R0
    VAND    V1.B16, V0.B16, V4.B16   // V4 = V0 & 0x0f
    VUSHR   $4, V0.B16, V5.B16       // V5 = V0 >> 4
    VTBL    V4.B16, [V2.B16], V4.B16 // V4 = rowLo[V4]
    VTBL    V5.B16, [V3.B16], V5.B16 // V5 = rowHi[V5]
    VEOR    V4.B16, V5.B16, V6.B16   // V6 = V4 ^ V5
    VST1.P  [V6.B16], 16(R0)         // R0 = V6; R0 += 16

    SUBS    $1, R5, R5
    BNE     loop16
after16:
    ANDS    $15, R1, R1
    BEQ     done
tail:
    MOVBU   0(R0), R2          // x
    AND     $0x0F, R2, R5      // low = x & 0x0F
    MOVBU   (R3)(R5), R5       // loVal = rowLo[low]

    LSR     $4, R2, R2         // high = x >> 4
    MOVBU   (R4)(R2), R2       // hiVal = rowHi[high]

    EOR     R2, R5, R5         // out = lo ^ hi
    MOVB.P  R5, 1(R0)          // store, vector++

    SUBS    $1, R1
    BNE     tail
done:
    RET

// func OctVecMulAdd(x, y []byte, multiplier uint8)
//
// R0 is the x load pointer, R7 the x store pointer (see OctVecAdd).
TEXT ·OctVecMulAdd(SB), NOSPLIT|NOFRAME, $0-49
    MOVD   x_base+0(FP),  R0         // R0 = &x[0]
    MOVD   x_len+8(FP),   R2         // R2 = len(x)
    MOVD   y_base+24(FP), R1         // R1 = &y[0]
    MOVBU  multiplier+48(FP), R5     // R5 = multiplier
    MOVD   R0, R7

    MOVD   $·_OctMulLo(SB), R3       // R3 = base of OctMulLo
    MOVD   $·_OctMulHi(SB), R4       // R4 = base of OctMulHi
    LSL    $4, R5, R6                // R6 = multiplier * 16
    ADD    R6, R3, R3                // R3 = &OctMulLo[multiplier*16]
    ADD    R6, R4, R4                // R4 = &OctMulHi[multiplier*16]

    VLD1   (R3), [V2.B16]            // V2 = rowLo
    VLD1   (R4), [V3.B16]            // V3 = rowHi
    VMOVI  $0x0f, V15.B16            // V15 = 0x0F mask

    LSR    $6, R2, R6                // R6 = n / 64
    CBZ    R6, after64

loop64:
    VLD1.P 64(R1), [V16.B16, V17.B16, V18.B16, V19.B16] // y
    VLD1.P 64(R0), [V20.B16, V21.B16, V22.B16, V23.B16] // x

    VAND   V15.B16, V16.B16, V4.B16  // low nibbles of y
    VUSHR  $4, V16.B16, V16.B16      // high nibbles of y
    VAND   V15.B16, V17.B16, V5.B16
    VUSHR  $4, V17.B16, V17.B16
    VAND   V15.B16, V18.B16, V6.B16
    VUSHR  $4, V18.B16, V18.B16
    VAND   V15.B16, V19.B16, V7.B16
    VUSHR  $4, V19.B16, V19.B16

    VTBL   V4.B16, [V2.B16], V4.B16
    VTBL   V16.B16, [V3.B16], V16.B16
    VTBL   V5.B16, [V2.B16], V5.B16
    VTBL   V17.B16, [V3.B16], V17.B16
    VTBL   V6.B16, [V2.B16], V6.B16
    VTBL   V18.B16, [V3.B16], V18.B16
    VTBL   V7.B16, [V2.B16], V7.B16
    VTBL   V19.B16, [V3.B16], V19.B16

    VEOR   V4.B16, V16.B16, V4.B16   // mul(y0..y3)
    VEOR   V5.B16, V17.B16, V5.B16
    VEOR   V6.B16, V18.B16, V6.B16
    VEOR   V7.B16, V19.B16, V7.B16

    VEOR   V20.B16, V4.B16, V4.B16   // x ^= mul(y)
    VEOR   V21.B16, V5.B16, V5.B16
    VEOR   V22.B16, V6.B16, V6.B16
    VEOR   V23.B16, V7.B16, V7.B16
    VST1.P [V4.B16, V5.B16, V6.B16, V7.B16], 64(R7)

    SUBS   $1, R6
    BNE    loop64
after64:
    ANDS   $63, R2, R2
    BEQ    done
    TBZ    $5, R2, after32

    // one 32B block
    VLD1.P 32(R1), [V0.B16, V1.B16]  // y0, y1
    VLD1.P 32(R0), [V4.B16, V5.B16]  // x0, x1

    VAND   V15.B16, V0.B16, V6.B16   // V6 = y0 & 0x0F
    VUSHR  $4,      V0.B16, V0.B16   // V0 = y0 >> 4
    VAND   V15.B16, V1.B16, V7.B16   // V7 = y1 & 0x0F
    VUSHR  $4,      V1.B16, V1.B16   // V1 = y1 >> 4

    VTBL   V6.B16, [V2.B16], V6.B16  // V6 = rowLo[low0]
    VTBL   V7.B16, [V2.B16], V7.B16  // V7 = rowLo[low1]
    VTBL   V0.B16, [V3.B16], V0.B16  // V0 = rowHi[high0]
    VTBL   V1.B16, [V3.B16], V1.B16  // V1 = rowHi[high1]

    VEOR   V6.B16, V0.B16, V6.B16    // V6 = mul(y0)
    VEOR   V7.B16, V1.B16, V7.B16    // V7 = mul(y1)

    VEOR   V6.B16, V4.B16, V4.B16    // x0 ^= mul(y0)
    VEOR   V7.B16, V5.B16, V5.B16    // x1 ^= mul(y1)
    VST1.P [V4.B16, V5.B16], 32(R7)

    AND    $31, R2, R2
    CBZ    R2, done
after32:
    TBZ    $4, R2, tail

    // one 16B block
    VLD1.P 16(R1), [V0.B16]
    VLD1.P 16(R0), [V4.B16]
    VAND   V15.B16, V0.B16, V6.B16
    VUSHR  $4, V0.B16, V0.B16
    VTBL   V6.B16, [V2.B16], V6.B16
    VTBL   V0.B16, [V3.B16], V0.B16
    VEOR   V6.B16, V0.B16, V6.B16
    VEOR   V6.B16, V4.B16, V4.B16
    VST1.P [V4.B16], 16(R7)

    AND    $15, R2, R2
    CBZ    R2, done

tail:
    MOVBU.P 1(R1), R8               // R8 = y[i], y++
    AND    $0x0F, R8, R9            // R9 = y[i] & 0x0F
    MOVBU  (R3)(R9), R9             // R9 = rowLo[lo]

    LSR    $4, R8, R8               // R8 = y[i] >> 4
    MOVBU  (R4)(R8), R8             // R8 = rowHi[hi]

    EOR    R8, R9, R9               // R9 = mul(y[i])

    MOVBU  0(R0), R10               // R10 = x[i]
    EOR    R9, R10, R10             // R10 = x[i] ^ mul(y[i])
    MOVB.P R10, 1(R0)               // store, x++

    SUBS   $1, R2
    BNE    tail

done:
    RET
