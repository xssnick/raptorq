//go:build amd64
#include "textflag.h"

// func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)
TEXT ·cpuid(SB), NOSPLIT, $0-24
    MOVL eaxArg+0(FP), AX
    MOVL ecxArg+4(FP), CX
    CPUID
    MOVL AX, eax+8(FP)
    MOVL BX, ebx+12(FP)
    MOVL CX, ecx+16(FP)
    MOVL DX, edx+20(FP)
    RET

// func xgetbv() (eax, edx uint32)
TEXT ·xgetbv(SB), NOSPLIT, $0-8
    MOVL $0, CX
    XGETBV
    MOVL AX, eax+0(FP)
    MOVL DX, edx+4(FP)
    RET

// func asmSSE2XORBlocks(x, y unsafe.Pointer, blocks int)
TEXT ·asmSSE2XORBlocks(SB), NOSPLIT, $0-24
    // x  -> AX (0(FP))
    // y  -> CX (8(FP))
    // blocks -> DX (16(FP))
    MOVQ x+0(FP), AX
    MOVQ y+8(FP), CX
    MOVQ blocks+16(FP), DX

    TESTQ DX, DX
    JZ done

loop:
    MOVOU (CX), X0
    MOVOU (AX), X1
    PXOR X0, X1
    MOVOU X1, (AX)

    ADDQ $16, AX
    ADDQ $16, CX
    DECQ DX
    JNZ loop

done:
    RET

// func asmSSSE3MulAdd(x, y unsafe.Pointer, table unsafe.Pointer, blocks int)
TEXT ·asmSSSE3MulAdd(SB), NOSPLIT, $0-32
    // x -> AX (0(FP))
    // y -> BX (8(FP))
    // table -> CX (16(FP))
    // blocks -> DX (24(FP))
    MOVQ x+0(FP), AX
    MOVQ y+8(FP), BX
    MOVQ table+16(FP), CX
    MOVQ blocks+24(FP), DX

    MOVOU (CX), X0        // low table
    MOVOU 16(CX), X1      // high table
    MOVOU ·mask<>(SB), X2 // mask 0x0F

    XORQ SI, SI

loopMulAdd:
    CMPQ SI, DX
    JGE doneMulAdd

    MOVQ SI, R8
    SHLQ $4, R8

    LEAQ (BX)(R8*1), R9
    MOVOU (R9), X3        // load y
    MOVOU X3, X4          // copy for high nibble

    LEAQ (AX)(R8*1), R10
    MOVOU (R10), X5       // load x

    PAND X2, X3           // low nibble indices
    MOVOU X0, X6
    PSHUFB X3, X6         // low table lookup

    PSRLW $4, X4
    PAND X2, X4
    MOVOU X1, X7
    PSHUFB X4, X7         // high table lookup

    PXOR X7, X6
    PXOR X6, X5
    MOVOU X5, (R10)       // store

    INCQ SI
    JMP loopMulAdd

doneMulAdd:
    RET

// func asmSSSE3Mul(x unsafe.Pointer, table unsafe.Pointer, blocks int)
TEXT ·asmSSSE3Mul(SB), NOSPLIT, $0-24
    // x -> AX (0(FP))
    // table -> CX (8(FP))
    // blocks -> DX (16(FP))
    MOVQ x+0(FP), AX
    MOVQ table+8(FP), CX
    MOVQ blocks+16(FP), DX

    MOVOU (CX), X0        // low table
    MOVOU 16(CX), X1      // high table
    MOVOU ·mask<>(SB), X2 // mask 0x0F

    XORQ SI, SI

loopMul:
    CMPQ SI, DX
    JGE doneMul

    MOVQ SI, R8
    SHLQ $4, R8

    LEAQ (AX)(R8*1), R9
    MOVOU (R9), X3        // load x
    MOVOU X3, X4          // copy for high nibble

    PAND X2, X3           // low nibble indices
    MOVOU X0, X5
    PSHUFB X3, X5         // low table lookup

    PSRLW $4, X4
    PAND X2, X4
    MOVOU X1, X6
    PSHUFB X4, X6         // high table lookup

    PXOR X6, X5
    MOVOU X5, (R9)        // store

    INCQ SI
    JMP loopMul

doneMul:
    RET

// func asmAVX2XORBlocks(x, y unsafe.Pointer, blocks int)
TEXT ·asmAVX2XORBlocks(SB), NOSPLIT, $0-24
    // x -> AX (0(FP))
    // y -> CX (8(FP))
    // blocks -> DX (16(FP))
    MOVQ x+0(FP), AX
    MOVQ y+8(FP), CX
    MOVQ blocks+16(FP), DX

    TESTQ DX, DX
    JZ doneAVX2XOR

loopAVX2XOR:
    VMOVDQU (CX), Y0
    VPXOR (AX), Y0, Y0
    VMOVDQU Y0, (AX)

    ADDQ $32, AX
    ADDQ $32, CX
    DECQ DX
    JNZ loopAVX2XOR

    VZEROUPPER

doneAVX2XOR:
    RET

DATA ·mask<>+0(SB)/8, $0x0f0f0f0f0f0f0f0f
DATA ·mask<>+8(SB)/8, $0x0f0f0f0f0f0f0f0f
GLOBL ·mask<>(SB), RODATA, $16
