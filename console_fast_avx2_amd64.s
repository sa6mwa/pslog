#include "textflag.h"

DATA ·avxConsoleHighBitMask+0(SB)/8, $0x8080808080808080
DATA ·avxConsoleHighBitMask+8(SB)/8, $0x8080808080808080
DATA ·avxConsoleHighBitMask+16(SB)/8, $0x8080808080808080
DATA ·avxConsoleHighBitMask+24(SB)/8, $0x8080808080808080
GLOBL ·avxConsoleHighBitMask(SB), RODATA, $32

DATA ·avxConsoleMinus20Mask+0(SB)/8, $0xe0e0e0e0e0e0e0e0
DATA ·avxConsoleMinus20Mask+8(SB)/8, $0xe0e0e0e0e0e0e0e0
DATA ·avxConsoleMinus20Mask+16(SB)/8, $0xe0e0e0e0e0e0e0e0
DATA ·avxConsoleMinus20Mask+24(SB)/8, $0xe0e0e0e0e0e0e0e0
GLOBL ·avxConsoleMinus20Mask(SB), RODATA, $32

DATA ·avxConsoleQuoteMask+0(SB)/8, $0x2222222222222222
DATA ·avxConsoleQuoteMask+8(SB)/8, $0x2222222222222222
DATA ·avxConsoleQuoteMask+16(SB)/8, $0x2222222222222222
DATA ·avxConsoleQuoteMask+24(SB)/8, $0x2222222222222222
GLOBL ·avxConsoleQuoteMask(SB), RODATA, $32

DATA ·avxConsoleBackslashMask+0(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA ·avxConsoleBackslashMask+8(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA ·avxConsoleBackslashMask+16(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA ·avxConsoleBackslashMask+24(SB)/8, $0x5c5c5c5c5c5c5c5c
GLOBL ·avxConsoleBackslashMask(SB), RODATA, $32

DATA ·avxConsoleSpaceMask+0(SB)/8, $0x2020202020202020
DATA ·avxConsoleSpaceMask+8(SB)/8, $0x2020202020202020
DATA ·avxConsoleSpaceMask+16(SB)/8, $0x2020202020202020
DATA ·avxConsoleSpaceMask+24(SB)/8, $0x2020202020202020
GLOBL ·avxConsoleSpaceMask(SB), RODATA, $32

DATA ·avxConsoleDelMask+0(SB)/8, $0x7f7f7f7f7f7f7f7f
DATA ·avxConsoleDelMask+8(SB)/8, $0x7f7f7f7f7f7f7f7f
DATA ·avxConsoleDelMask+16(SB)/8, $0x7f7f7f7f7f7f7f7f
DATA ·avxConsoleDelMask+24(SB)/8, $0x7f7f7f7f7f7f7f7f
GLOBL ·avxConsoleDelMask(SB), RODATA, $32

// func firstConsoleUnsafeIndexAVX2(s string) int
TEXT ·firstConsoleUnsafeIndexAVX2(SB), NOSPLIT, $0-24
    MOVQ s_base+0(FP), SI
    MOVQ s_len+8(FP), CX
    XORQ AX, AX
    TESTQ CX, CX
    JE   done

    VMOVDQU ·avxConsoleHighBitMask(SB), Y4
    VMOVDQU ·avxConsoleMinus20Mask(SB), Y5
    VMOVDQU ·avxConsoleQuoteMask(SB), Y6
    VMOVDQU ·avxConsoleBackslashMask(SB), Y7
    VMOVDQU ·avxConsoleSpaceMask(SB), Y8
    VMOVDQU ·avxConsoleDelMask(SB), Y9

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

    VPCMPEQB Y3, Y0, Y8
    VPMOVMSKB Y3, R12

    VPCMPEQB Y3, Y0, Y9
    VPMOVMSKB Y3, R13

    ORL R9, R8
    ORL R10, R8
    ORL R11, R8
    ORL R12, R8
    ORL R13, R8
    TESTL R8, R8
    JZ   continue32

    MOVL R8, R14
    BSFL R14, R14
    ADDQ R14, SI
    SUBQ R14, CX
    ADDQ R14, AX
    VZEROUPPER
prepare_tail:
    MOVQ $0x8080808080808080, R8
    MOVQ $0x2020202020202020, R9
    MOVQ $0x0101010101010101, R10
    MOVQ $0x2222222222222222, R11
    MOVQ $0x5c5c5c5c5c5c5c5c, R12
    MOVQ $0x2020202020202020, R13
    MOVQ $0x7f7f7f7f7f7f7f7f, R14
    JMP  tail_loop

continue32:
    ADDQ $32, SI
    ADDQ $32, AX
    SUBQ $32, CX
    JMP  loop32

prepare_chunk:
    VZEROUPPER
    MOVQ $0x8080808080808080, R8
    MOVQ $0x2020202020202020, R9
    MOVQ $0x0101010101010101, R10
    MOVQ $0x2222222222222222, R11
    MOVQ $0x5c5c5c5c5c5c5c5c, R12
    MOVQ $0x2020202020202020, R13
    MOVQ $0x7f7f7f7f7f7f7f7f, R14

chunk_loop:
    CMPQ CX, $16
    JL   chunk8

    MOVQ (SI), R15
    MOVQ R15, BX
    ANDQ R8, BX
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    SUBQ R9, DX
    MOVQ R15, BP
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R11, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R12, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R13, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R14, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ 8(SI), R15
    MOVQ R15, BX
    ANDQ R8, BX
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    SUBQ R9, DX
    MOVQ R15, BP
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R11, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R12, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R13, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R14, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    ADDQ $16, SI
    ADDQ $16, AX
    SUBQ $16, CX
    JMP  chunk_loop

chunk8:
    CMPQ CX, $8
    JL   tail_loop

    MOVQ (SI), R15
    MOVQ R15, BX
    ANDQ R8, BX
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    SUBQ R9, DX
    MOVQ R15, BP
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R11, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R12, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R13, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    MOVQ R15, DX
    XORQ R14, DX
    MOVQ DX, BP
    SUBQ R10, DX
    NOTQ BP
    ANDQ DX, BP
    ANDQ R8, BP
    JNE  chunk_has_unsafe

    ADDQ $8, SI
    ADDQ $8, AX
    SUBQ $8, CX
    JMP  chunk_loop

chunk_has_unsafe:
    // examine bytes

tail_loop:
    TESTQ CX, CX
    JE   done

tail_iter:
    MOVBQZX (SI), R15
    CMPB  R15, $0x80
    JAE   found
    CMPB  R15, $0x20
    JBE   found
    CMPB  R15, $0x22
    JE    found
    CMPB  R15, $0x5c
    JE    found
    CMPB  R15, $0x7f
    JAE   found

    INCQ SI
    INCQ AX
    DECQ CX
    CMPQ CX, $8
    JGE  chunk8
    TESTQ CX, CX
    JNZ  tail_iter
    JMP  done

found:
    MOVQ AX, ret+16(FP)
    RET

done:
    MOVQ AX, ret+16(FP)
    RET
