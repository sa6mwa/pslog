#include "textflag.h"

DATA ·avxJsonHighBitMask+0(SB)/8, $0x8080808080808080
DATA ·avxJsonHighBitMask+8(SB)/8, $0x8080808080808080
DATA ·avxJsonHighBitMask+16(SB)/8, $0x8080808080808080
DATA ·avxJsonHighBitMask+24(SB)/8, $0x8080808080808080
GLOBL ·avxJsonHighBitMask(SB), RODATA, $32

DATA ·avxJsonMinus20Mask+0(SB)/8, $0xe0e0e0e0e0e0e0e0
DATA ·avxJsonMinus20Mask+8(SB)/8, $0xe0e0e0e0e0e0e0e0
DATA ·avxJsonMinus20Mask+16(SB)/8, $0xe0e0e0e0e0e0e0e0
DATA ·avxJsonMinus20Mask+24(SB)/8, $0xe0e0e0e0e0e0e0e0
GLOBL ·avxJsonMinus20Mask(SB), RODATA, $32

DATA ·avxJsonQuoteMask+0(SB)/8, $0x2222222222222222
DATA ·avxJsonQuoteMask+8(SB)/8, $0x2222222222222222
DATA ·avxJsonQuoteMask+16(SB)/8, $0x2222222222222222
DATA ·avxJsonQuoteMask+24(SB)/8, $0x2222222222222222
GLOBL ·avxJsonQuoteMask(SB), RODATA, $32

DATA ·avxJsonBackslashMask+0(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA ·avxJsonBackslashMask+8(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA ·avxJsonBackslashMask+16(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA ·avxJsonBackslashMask+24(SB)/8, $0x5c5c5c5c5c5c5c5c
GLOBL ·avxJsonBackslashMask(SB), RODATA, $32

// func firstUnsafeIndexAVX2(s string) int
TEXT ·firstUnsafeIndexAVX2(SB), NOSPLIT, $0-24
    MOVQ s_base+0(FP), SI
    MOVQ s_len+8(FP), CX
    XORQ AX, AX
    TESTQ CX, CX
    JE   done

    VMOVDQU ·avxJsonHighBitMask(SB), Y4
    VMOVDQU ·avxJsonMinus20Mask(SB), Y5
    VMOVDQU ·avxJsonQuoteMask(SB), Y6
    VMOVDQU ·avxJsonBackslashMask(SB), Y7

loop32:
    CMPQ CX, $32
    JL   prepare_chunk

    VMOVDQU (SI), Y0
    VPMOVMSKB Y0, R8

    VPADDB Y1, Y0, Y5
    VPMOVMSKB Y1, R9

    VPCMPEQB Y2, Y0, Y6
    VPMOVMSKB Y2, R10

    VPCMPEQB Y3, Y0, Y7
    VPMOVMSKB Y3, R11

    ORL R9, R8
    ORL R10, R8
    ORL R11, R8
    TESTL R8, R8
    JZ   continue32

    MOVL R8, R12
    BSFL R12, R12
    ADDQ R12, SI
    SUBQ R12, CX
    ADDQ R12, AX
    VZEROUPPER
prepare_tail:
    MOVQ $0x8080808080808080, R13
    MOVQ $0x2020202020202020, R14
    MOVQ $0x0101010101010101, R15
    MOVQ $0x2222222222222222, BX
    MOVQ $0x5c5c5c5c5c5c5c5c, DI
    JMP  tail_loop

continue32:
    ADDQ $32, SI
    ADDQ $32, AX
    SUBQ $32, CX
    JMP  loop32

prepare_chunk:
    VZEROUPPER
    MOVQ $0x8080808080808080, R13
    MOVQ $0x2020202020202020, R14
    MOVQ $0x0101010101010101, R15
    MOVQ $0x2222222222222222, BX
    MOVQ $0x5c5c5c5c5c5c5c5c, DI

chunk_loop:
    CMPQ CX, $16
    JL   chunk8

    MOVQ (SI), DX
    MOVQ DX, R8
    ANDQ R13, R8
    JNE  chunk_has_unsafe

    MOVQ DX, R9
    SUBQ R14, R9
    MOVQ DX, R10
    NOTQ R10
    ANDQ R9, R10
    ANDQ R13, R10
    JNE  chunk_has_unsafe

    MOVQ DX, R11
    XORQ BX, R11
    MOVQ R11, R12
    SUBQ R15, R11
    NOTQ R12
    ANDQ R11, R12
    ANDQ R13, R12
    JNE  chunk_has_unsafe

    MOVQ DX, R11
    XORQ DI, R11
    MOVQ R11, R12
    SUBQ R15, R11
    NOTQ R12
    ANDQ R11, R12
    ANDQ R13, R12
    JNE  chunk_has_unsafe

    ADDQ $8, SI
    ADDQ $8, AX
    SUBQ $8, CX

    MOVQ (SI), DX
    MOVQ DX, R8
    ANDQ R13, R8
    JNE  chunk_has_unsafe

    MOVQ DX, R9
    SUBQ R14, R9
    MOVQ DX, R10
    NOTQ R10
    ANDQ R9, R10
    ANDQ R13, R10
    JNE  chunk_has_unsafe

    MOVQ DX, R11
    XORQ BX, R11
    MOVQ R11, R12
    SUBQ R15, R11
    NOTQ R12
    ANDQ R11, R12
    ANDQ R13, R12
    JNE  chunk_has_unsafe

    MOVQ DX, R11
    XORQ DI, R11
    MOVQ R11, R12
    SUBQ R15, R11
    NOTQ R12
    ANDQ R11, R12
    ANDQ R13, R12
    JNE  chunk_has_unsafe

    ADDQ $8, SI
    ADDQ $8, AX
    SUBQ $8, CX
    JMP  chunk_loop

chunk8:
    CMPQ CX, $8
    JL   tail_loop

    MOVQ (SI), DX
    MOVQ DX, R8
    ANDQ R13, R8
    JNE  chunk_has_unsafe

    MOVQ DX, R9
    SUBQ R14, R9
    MOVQ DX, R10
    NOTQ R10
    ANDQ R9, R10
    ANDQ R13, R10
    JNE  chunk_has_unsafe

    MOVQ DX, R11
    XORQ BX, R11
    MOVQ R11, R12
    SUBQ R15, R11
    NOTQ R12
    ANDQ R11, R12
    ANDQ R13, R12
    JNE  chunk_has_unsafe

    MOVQ DX, R11
    XORQ DI, R11
    MOVQ R11, R12
    SUBQ R15, R11
    NOTQ R12
    ANDQ R11, R12
    ANDQ R13, R12
    JNE  chunk_has_unsafe

    ADDQ $8, SI
    ADDQ $8, AX
    SUBQ $8, CX
    JMP  chunk_loop

chunk_has_unsafe:
    // fallthrough to tail loop

tail_loop:
    TESTQ CX, CX
    JE   done

tail_iter:
    MOVBQZX (SI), DX
    CMPB  DL, $0x80
    JAE   tail_safe
    CMPB  DL, $0x20
    JB    found
    CMPB  DL, $0x22 // '"'
    JE    found
    CMPB  DL, $0x5c // '\\'
    JE    found

tail_safe:
    INCQ SI
    INCQ AX
    DECQ CX
    CMPQ CX, $8
    JGE  chunk_loop
    TESTQ CX, CX
    JNZ  tail_iter
    JMP  done

found:
    MOVQ AX, ret+16(FP)
    RET

done:
    MOVQ AX, ret+16(FP)
    RET
