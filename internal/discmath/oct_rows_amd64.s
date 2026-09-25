//go:build amd64 && !purego
#include "textflag.h"

// The SSE2/SSSE3 counterparts of oct_rows_arm64.s, see there. PSHUFB needs
// SSSE3, which OctVecMulAdd already assumes.

// func octAddRowsAsm(dst, init, base *byte, stride, n int, idx *uint32, k int)
// n is a positive multiple of 16. dst = init ^ row(idx[0]) ^ ... ^ row(idx[k-1])
// where row(i) = base + i*stride; a nil init counts as zero, init may be dst.
TEXT ·octAddRowsAsm(SB), NOSPLIT, $0-56
    MOVQ  dst+0(FP), DI
    MOVQ  init+8(FP), SI
    MOVQ  base+16(FP), BX
    MOVQ  stride+24(FP), R8
    MOVQ  n+32(FP), CX
    MOVQ  idx+40(FP), R9
    MOVQ  k+48(FP), R10
    XORQ  AX, AX                  // AX = offset of the current block
    MOVQ  CX, R11
    ANDQ  $-128, R11              // R11 = end of the 128B blocks

add128:
    CMPQ  AX, R11
    JEQ   add16
    TESTQ SI, SI
    JZ    zero128
    MOVOU 0(SI)(AX*1), X0
    MOVOU 16(SI)(AX*1), X1
    MOVOU 32(SI)(AX*1), X2
    MOVOU 48(SI)(AX*1), X3
    MOVOU 64(SI)(AX*1), X4
    MOVOU 80(SI)(AX*1), X5
    MOVOU 96(SI)(AX*1), X6
    MOVOU 112(SI)(AX*1), X7
    JMP   src128
zero128:
    PXOR  X0, X0
    PXOR  X1, X1
    PXOR  X2, X2
    PXOR  X3, X3
    PXOR  X4, X4
    PXOR  X5, X5
    PXOR  X6, X6
    PXOR  X7, X7
src128:
    XORQ  R12, R12
    TESTQ R10, R10
    JZ    store128
srcloop128:
    MOVL  (R9)(R12*4), R13        // row index
    IMULQ R8, R13
    ADDQ  BX, R13
    MOVOU 0(R13)(AX*1), X8
    MOVOU 16(R13)(AX*1), X9
    MOVOU 32(R13)(AX*1), X10
    MOVOU 48(R13)(AX*1), X11
    MOVOU 64(R13)(AX*1), X12
    MOVOU 80(R13)(AX*1), X13
    MOVOU 96(R13)(AX*1), X14
    MOVOU 112(R13)(AX*1), X15
    PXOR  X8, X0
    PXOR  X9, X1
    PXOR  X10, X2
    PXOR  X11, X3
    PXOR  X12, X4
    PXOR  X13, X5
    PXOR  X14, X6
    PXOR  X15, X7
    INCQ  R12
    CMPQ  R12, R10
    JNE   srcloop128
store128:
    MOVOU X0, 0(DI)(AX*1)
    MOVOU X1, 16(DI)(AX*1)
    MOVOU X2, 32(DI)(AX*1)
    MOVOU X3, 48(DI)(AX*1)
    MOVOU X4, 64(DI)(AX*1)
    MOVOU X5, 80(DI)(AX*1)
    MOVOU X6, 96(DI)(AX*1)
    MOVOU X7, 112(DI)(AX*1)
    ADDQ  $128, AX
    JMP   add128

add16:
    CMPQ  AX, CX
    JEQ   addDone
    TESTQ SI, SI
    JZ    zero16
    MOVOU (SI)(AX*1), X0
    JMP   src16
zero16:
    PXOR  X0, X0
src16:
    XORQ  R12, R12
    TESTQ R10, R10
    JZ    store16
srcloop16:
    MOVL  (R9)(R12*4), R13
    IMULQ R8, R13
    ADDQ  BX, R13
    MOVOU (R13)(AX*1), X8
    PXOR  X8, X0
    INCQ  R12
    CMPQ  R12, R10
    JNE   srcloop16
store16:
    MOVOU X0, (DI)(AX*1)
    ADDQ  $16, AX
    JMP   add16
addDone:
    RET

// func octMulAddRowsAsm(dst, base *byte, stride, n int, idx *uint32, muls *byte, k int)
// n is a positive multiple of 16, k > 0.
// dst ^= muls[0]*row(idx[0]) ^ ... ^ muls[k-1]*row(idx[k-1]), row(i) = base + i*stride
TEXT ·octMulAddRowsAsm(SB), NOSPLIT, $0-56
    MOVQ  dst+0(FP), DI
    MOVQ  base+8(FP), BX
    MOVQ  stride+16(FP), R8
    MOVQ  n+24(FP), CX
    MOVQ  idx+32(FP), R9
    MOVQ  muls+40(FP), SI
    MOVQ  k+48(FP), R10
    LEAQ  ·_Mul4bitPreCalc(SB), DX  // [256][32]: 16 low then 16 high nibble products
    MOVOU ·rowsNibbleMask<>(SB), X14
    XORQ  AX, AX
    MOVQ  CX, R11
    ANDQ  $-64, R11               // R11 = end of the 64B blocks

mul64:
    CMPQ  AX, R11
    JEQ   mul16
    MOVOU 0(DI)(AX*1), X0
    MOVOU 16(DI)(AX*1), X1
    MOVOU 32(DI)(AX*1), X2
    MOVOU 48(DI)(AX*1), X3
    XORQ  R12, R12
msrc64:
    MOVBQZX (SI)(R12*1), R14
    SHLQ  $5, R14
    MOVOU (DX)(R14*1), X12
    MOVOU 16(DX)(R14*1), X13
    MOVL  (R9)(R12*4), R13
    IMULQ R8, R13
    ADDQ  BX, R13
    MOVOU 0(R13)(AX*1), X4
    MOVOU X4, X5
    PSRLQ $4, X5
    PAND  X14, X4                 // low nibbles
    PAND  X14, X5                 // high nibbles
    MOVOU X12, X6
    PSHUFB X4, X6
    MOVOU X13, X7
    PSHUFB X5, X7
    PXOR  X6, X7
    PXOR  X7, X0
    MOVOU 16(R13)(AX*1), X4
    MOVOU X4, X5
    PSRLQ $4, X5
    PAND  X14, X4                 // low nibbles
    PAND  X14, X5                 // high nibbles
    MOVOU X12, X6
    PSHUFB X4, X6
    MOVOU X13, X7
    PSHUFB X5, X7
    PXOR  X6, X7
    PXOR  X7, X1
    MOVOU 32(R13)(AX*1), X4
    MOVOU X4, X5
    PSRLQ $4, X5
    PAND  X14, X4                 // low nibbles
    PAND  X14, X5                 // high nibbles
    MOVOU X12, X6
    PSHUFB X4, X6
    MOVOU X13, X7
    PSHUFB X5, X7
    PXOR  X6, X7
    PXOR  X7, X2
    MOVOU 48(R13)(AX*1), X4
    MOVOU X4, X5
    PSRLQ $4, X5
    PAND  X14, X4                 // low nibbles
    PAND  X14, X5                 // high nibbles
    MOVOU X12, X6
    PSHUFB X4, X6
    MOVOU X13, X7
    PSHUFB X5, X7
    PXOR  X6, X7
    PXOR  X7, X3
    INCQ  R12
    CMPQ  R12, R10
    JNE   msrc64
    MOVOU X0, 0(DI)(AX*1)
    MOVOU X1, 16(DI)(AX*1)
    MOVOU X2, 32(DI)(AX*1)
    MOVOU X3, 48(DI)(AX*1)
    ADDQ  $64, AX
    JMP   mul64

mul16:
    CMPQ  AX, CX
    JEQ   mulDone
    MOVOU (DI)(AX*1), X0
    XORQ  R12, R12
msrc16:
    MOVBQZX (SI)(R12*1), R14
    SHLQ  $5, R14
    MOVOU (DX)(R14*1), X12
    MOVOU 16(DX)(R14*1), X13
    MOVL  (R9)(R12*4), R13
    IMULQ R8, R13
    ADDQ  BX, R13
    MOVOU 0(R13)(AX*1), X4
    MOVOU X4, X5
    PSRLQ $4, X5
    PAND  X14, X4                 // low nibbles
    PAND  X14, X5                 // high nibbles
    MOVOU X12, X6
    PSHUFB X4, X6
    MOVOU X13, X7
    PSHUFB X5, X7
    PXOR  X6, X7
    PXOR  X7, X0
    INCQ  R12
    CMPQ  R12, R10
    JNE   msrc16
    MOVOU X0, (DI)(AX*1)
    ADDQ  $16, AX
    JMP   mul16
mulDone:
    RET

// func octHdpcStepAsm(acc, v, ua, ub *byte, n int)
// n is a positive multiple of 16, v may be nil (a zero row).
// acc = 2*acc ^ v; ua ^= acc; ub ^= acc, 2*x = x<<1 ^ (x&0x80 ? 0x1d : 0)
TEXT ·octHdpcStepAsm(SB), NOSPLIT, $0-40
    MOVQ  acc+0(FP), DI
    MOVQ  v+8(FP), SI
    MOVQ  ua+16(FP), BX
    MOVQ  ub+24(FP), R8
    MOVQ  n+32(FP), CX
    MOVOU ·rowsPoly<>(SB), X15
    XORQ  AX, AX
    MOVQ  CX, R11
    ANDQ  $-64, R11
    TESTQ SI, SI
    JZ    zero64

vec64:
    CMPQ  AX, R11
    JEQ   vec16
    MOVOU 0(DI)(AX*1), X0
    PXOR  X4, X4
    PCMPGTB X0, X4            // 0xff where the top bit is set
    PAND  X15, X4
    PADDB X0, X0              // x<<1
    PXOR  X4, X0
    MOVOU 0(SI)(AX*1), X4
    PXOR  X4, X0
    MOVOU X0, 0(DI)(AX*1)
    MOVOU 0(BX)(AX*1), X4
    PXOR  X0, X4
    MOVOU X4, 0(BX)(AX*1)
    MOVOU 0(R8)(AX*1), X4
    PXOR  X0, X4
    MOVOU X4, 0(R8)(AX*1)
    MOVOU 16(DI)(AX*1), X1
    PXOR  X5, X5
    PCMPGTB X1, X5            // 0xff where the top bit is set
    PAND  X15, X5
    PADDB X1, X1              // x<<1
    PXOR  X5, X1
    MOVOU 16(SI)(AX*1), X5
    PXOR  X5, X1
    MOVOU X1, 16(DI)(AX*1)
    MOVOU 16(BX)(AX*1), X5
    PXOR  X1, X5
    MOVOU X5, 16(BX)(AX*1)
    MOVOU 16(R8)(AX*1), X5
    PXOR  X1, X5
    MOVOU X5, 16(R8)(AX*1)
    MOVOU 32(DI)(AX*1), X2
    PXOR  X6, X6
    PCMPGTB X2, X6            // 0xff where the top bit is set
    PAND  X15, X6
    PADDB X2, X2              // x<<1
    PXOR  X6, X2
    MOVOU 32(SI)(AX*1), X6
    PXOR  X6, X2
    MOVOU X2, 32(DI)(AX*1)
    MOVOU 32(BX)(AX*1), X6
    PXOR  X2, X6
    MOVOU X6, 32(BX)(AX*1)
    MOVOU 32(R8)(AX*1), X6
    PXOR  X2, X6
    MOVOU X6, 32(R8)(AX*1)
    MOVOU 48(DI)(AX*1), X3
    PXOR  X7, X7
    PCMPGTB X3, X7            // 0xff where the top bit is set
    PAND  X15, X7
    PADDB X3, X3              // x<<1
    PXOR  X7, X3
    MOVOU 48(SI)(AX*1), X7
    PXOR  X7, X3
    MOVOU X3, 48(DI)(AX*1)
    MOVOU 48(BX)(AX*1), X7
    PXOR  X3, X7
    MOVOU X7, 48(BX)(AX*1)
    MOVOU 48(R8)(AX*1), X7
    PXOR  X3, X7
    MOVOU X7, 48(R8)(AX*1)
    ADDQ  $64, AX
    JMP   vec64
vec16:
    CMPQ  AX, CX
    JEQ   stepDone
    MOVOU 0(DI)(AX*1), X0
    PXOR  X4, X4
    PCMPGTB X0, X4            // 0xff where the top bit is set
    PAND  X15, X4
    PADDB X0, X0              // x<<1
    PXOR  X4, X0
    MOVOU 0(SI)(AX*1), X4
    PXOR  X4, X0
    MOVOU X0, 0(DI)(AX*1)
    MOVOU 0(BX)(AX*1), X4
    PXOR  X0, X4
    MOVOU X4, 0(BX)(AX*1)
    MOVOU 0(R8)(AX*1), X4
    PXOR  X0, X4
    MOVOU X4, 0(R8)(AX*1)
    ADDQ  $16, AX
    JMP   vec16

zero64:
    CMPQ  AX, R11
    JEQ   zero16
    MOVOU 0(DI)(AX*1), X0
    PXOR  X4, X4
    PCMPGTB X0, X4            // 0xff where the top bit is set
    PAND  X15, X4
    PADDB X0, X0              // x<<1
    PXOR  X4, X0
    MOVOU X0, 0(DI)(AX*1)
    MOVOU 0(BX)(AX*1), X4
    PXOR  X0, X4
    MOVOU X4, 0(BX)(AX*1)
    MOVOU 0(R8)(AX*1), X4
    PXOR  X0, X4
    MOVOU X4, 0(R8)(AX*1)
    MOVOU 16(DI)(AX*1), X1
    PXOR  X5, X5
    PCMPGTB X1, X5            // 0xff where the top bit is set
    PAND  X15, X5
    PADDB X1, X1              // x<<1
    PXOR  X5, X1
    MOVOU X1, 16(DI)(AX*1)
    MOVOU 16(BX)(AX*1), X5
    PXOR  X1, X5
    MOVOU X5, 16(BX)(AX*1)
    MOVOU 16(R8)(AX*1), X5
    PXOR  X1, X5
    MOVOU X5, 16(R8)(AX*1)
    MOVOU 32(DI)(AX*1), X2
    PXOR  X6, X6
    PCMPGTB X2, X6            // 0xff where the top bit is set
    PAND  X15, X6
    PADDB X2, X2              // x<<1
    PXOR  X6, X2
    MOVOU X2, 32(DI)(AX*1)
    MOVOU 32(BX)(AX*1), X6
    PXOR  X2, X6
    MOVOU X6, 32(BX)(AX*1)
    MOVOU 32(R8)(AX*1), X6
    PXOR  X2, X6
    MOVOU X6, 32(R8)(AX*1)
    MOVOU 48(DI)(AX*1), X3
    PXOR  X7, X7
    PCMPGTB X3, X7            // 0xff where the top bit is set
    PAND  X15, X7
    PADDB X3, X3              // x<<1
    PXOR  X7, X3
    MOVOU X3, 48(DI)(AX*1)
    MOVOU 48(BX)(AX*1), X7
    PXOR  X3, X7
    MOVOU X7, 48(BX)(AX*1)
    MOVOU 48(R8)(AX*1), X7
    PXOR  X3, X7
    MOVOU X7, 48(R8)(AX*1)
    ADDQ  $64, AX
    JMP   zero64
zero16:
    CMPQ  AX, CX
    JEQ   stepDone
    MOVOU 0(DI)(AX*1), X0
    PXOR  X4, X4
    PCMPGTB X0, X4            // 0xff where the top bit is set
    PAND  X15, X4
    PADDB X0, X0              // x<<1
    PXOR  X4, X0
    MOVOU X0, 0(DI)(AX*1)
    MOVOU 0(BX)(AX*1), X4
    PXOR  X0, X4
    MOVOU X4, 0(BX)(AX*1)
    MOVOU 0(R8)(AX*1), X4
    PXOR  X0, X4
    MOVOU X4, 0(R8)(AX*1)
    ADDQ  $16, AX
    JMP   zero16
stepDone:
    RET

DATA ·rowsNibbleMask<>+0(SB)/8, $0x0f0f0f0f0f0f0f0f
DATA ·rowsNibbleMask<>+8(SB)/8, $0x0f0f0f0f0f0f0f0f
GLOBL ·rowsNibbleMask<>(SB), RODATA, $16

DATA ·rowsPoly<>+0(SB)/8, $0x1d1d1d1d1d1d1d1d
DATA ·rowsPoly<>+8(SB)/8, $0x1d1d1d1d1d1d1d1d
GLOBL ·rowsPoly<>(SB), RODATA, $16
