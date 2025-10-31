#include "textflag.h"

DATA ·hexDigits+0(SB)/16, $"0123456789abcdef"
GLOBL ·hexDigits(SB), RODATA|NOPTR, $16

DATA ·avxJsonHighBitMask+0(SB)/8, $0x8080808080808080
DATA ·avxJsonHighBitMask+8(SB)/8, $0x8080808080808080
DATA ·avxJsonHighBitMask+16(SB)/8, $0x8080808080808080
DATA ·avxJsonHighBitMask+24(SB)/8, $0x8080808080808080
GLOBL ·avxJsonHighBitMask(SB), RODATA|NOPTR, $32

DATA ·avxJsonMinus20Mask+0(SB)/8, $0xe0e0e0e0e0e0e0e0
DATA ·avxJsonMinus20Mask+8(SB)/8, $0xe0e0e0e0e0e0e0e0
DATA ·avxJsonMinus20Mask+16(SB)/8, $0xe0e0e0e0e0e0e0e0
DATA ·avxJsonMinus20Mask+24(SB)/8, $0xe0e0e0e0e0e0e0e0
GLOBL ·avxJsonMinus20Mask(SB), RODATA|NOPTR, $32

DATA ·avxJsonQuoteMask+0(SB)/8, $0x2222222222222222
DATA ·avxJsonQuoteMask+8(SB)/8, $0x2222222222222222
DATA ·avxJsonQuoteMask+16(SB)/8, $0x2222222222222222
DATA ·avxJsonQuoteMask+24(SB)/8, $0x2222222222222222
GLOBL ·avxJsonQuoteMask(SB), RODATA|NOPTR, $32

DATA ·avxJsonBackslashMask+0(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA ·avxJsonBackslashMask+8(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA ·avxJsonBackslashMask+16(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA ·avxJsonBackslashMask+24(SB)/8, $0x5c5c5c5c5c5c5c5c
GLOBL ·avxJsonBackslashMask(SB), RODATA|NOPTR, $32

// func quoteJSONStringAMD64(dst *byte, src *byte, n int) int
TEXT ·quoteJSONStringAMD64(SB), NOSPLIT, $0-32
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), CX
	XORQ AX, AX

	MOVB $'"', (DI)
	INCQ DI
	INCQ AX

	TESTQ CX, CX
	JE   finish_quote

	VMOVDQU ·avxJsonHighBitMask(SB), Y4
	VMOVDQU ·avxJsonMinus20Mask(SB), Y5
	VMOVDQU ·avxJsonQuoteMask(SB), Y6
	VMOVDQU ·avxJsonBackslashMask(SB), Y7
	LEAQ ·hexDigits(SB), R14

vector_loop:
	CMPQ CX, $32
	JL   scalar_loop

	VMOVDQU (SI), Y0
	VPMOVMSKB Y0, R8

	VPADDB Y1, Y0, Y5
	VPMOVMSKB Y1, R9

	VPCMPEQB Y2, Y0, Y6
	VPMOVMSKB Y2, R10

	VPCMPEQB Y3, Y0, Y7
	VPMOVMSKB Y3, R12

	ORL R9, R8
	ORL R10, R8
	ORL R12, R8
	TESTL R8, R8
	JNZ  vector_hit_unsafe

	VMOVDQU Y0, (DI)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $32, AX
	SUBQ $32, CX
	JMP  vector_loop

vector_hit_unsafe:
	MOVL R8, R9
	BSFL R9, R9
	MOVQ R9, R10

copy_prefix_loop:
	TESTQ R10, R10
	JE   scalar_loop
	MOVB (SI), R11
	MOVB R11B, (DI)
	INCQ SI
	INCQ DI
	INCQ AX
	DECQ CX
	DECQ R10
	JMP  copy_prefix_loop

scalar_loop:
	TESTQ CX, CX
	JE   finish_quote

scalar_iter:
	MOVBQZX (SI), BX
	INCQ SI
	DECQ CX

	TESTB $0x80, BL
	JNZ  copy_safe
	CMPB BL, $0x20
	JB   ctrl_escape
	CMPB BL, $0x22
	JE   quote_escape
	CMPB BL, $0x5c
	JE   backslash_escape
	CMPB BL, $0x7f
	JE   hex_escape

copy_safe:
	MOVB BL, (DI)
	INCQ DI
	INCQ AX
	CMPQ CX, $32
	JGE  vector_loop
	TESTQ CX, CX
	JG   scalar_iter
	JMP  finish_quote

quote_escape:
	MOVB $'\\', (DI)
	MOVB BL, 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  post_escape

backslash_escape:
	MOVB $'\\', (DI)
	MOVB BL, 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  post_escape

ctrl_escape:
	CMPB BL, $'\b'
	JE   escape_b
	CMPB BL, $'\f'
	JE   escape_f
	CMPB BL, $'\n'
	JE   escape_n
	CMPB BL, $'\r'
	JE   escape_r
	CMPB BL, $'\t'
	JE   escape_t
	JMP  hex_escape

escape_b:
	MOVB $'\\', (DI)
	MOVB $'b', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  post_escape

escape_f:
	MOVB $'\\', (DI)
	MOVB $'f', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  post_escape

escape_n:
	MOVB $'\\', (DI)
	MOVB $'n', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  post_escape

escape_r:
	MOVB $'\\', (DI)
	MOVB $'r', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  post_escape

escape_t:
	MOVB $'\\', (DI)
	MOVB $'t', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  post_escape

hex_escape:
	MOVB $'\\', (DI)
	MOVB $'x', 1(DI)

	MOVQ BX, R8
	SHRQ $4, R8
	MOVB (R14)(R8*1), R9B
	MOVB R9B, 2(DI)

	MOVQ BX, R10
	ANDQ $0x0f, R10
	MOVB (R14)(R10*1), R9B
	MOVB R9B, 3(DI)

	ADDQ $4, DI
	ADDQ $4, AX
	JMP  post_escape

post_escape:
	CMPQ CX, $32
	JGE  vector_loop
	TESTQ CX, CX
	JG   scalar_iter
	JMP  finish_quote

finish_quote:
	MOVB $'"', (DI)
	INCQ DI
	INCQ AX
	VZEROUPPER
	MOVQ AX, ret+24(FP)
	RET
