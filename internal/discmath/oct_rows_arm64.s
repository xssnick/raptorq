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
