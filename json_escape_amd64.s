#include "textflag.h"

DATA ·hexDigits+0(SB)/16, $"0123456789abcdef"
GLOBL ·hexDigits(SB), RODATA|NOPTR, $16

// func escapeJSONStringAMD64(dst *byte, src *byte, n int) int
TEXT ·escapeJSONStringAMD64(SB), NOSPLIT, $0-32
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), CX
	XORQ AX, AX
	LEAQ ·hexDigits(SB), R11

	VMOVDQU ·avxJsonHighBitMask(SB), Y4
	VMOVDQU ·avxJsonMinus20Mask(SB), Y5
	VMOVDQU ·avxJsonQuoteMask(SB), Y6
	VMOVDQU ·avxJsonBackslashMask(SB), Y7

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
	TESTQ R9, R9
	JE   scalar_loop

	MOVQ R9, R10

copy_prefix_loop:
	MOVB (SI), R13
	MOVB R13B, (DI)
	INCQ SI
	INCQ DI
	INCQ AX
	DECQ CX
	DECQ R10
	JNZ  copy_prefix_loop

check_vector:
	CMPQ CX, $32
	JGE  vector_loop

scalar_loop:
	CMPQ CX, $0
	JE   done

	MOVBQZX (SI), BX
	INCQ SI
	DECQ CX

	TESTB $0x80, BL
	JNZ  copy_safe_scalar
	CMPB BL, $0x20
	JB   ctrl_escape
	CMPB BL, $0x22
	JE   quote_escape
	CMPB BL, $0x5c
	JE   backslash_escape

copy_safe_scalar:
	MOVB BL, (DI)
	INCQ DI
	INCQ AX
	JMP  check_vector

quote_escape:
	MOVB $'\\', (DI)
	MOVB BL, 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  check_vector

backslash_escape:
	MOVB $'\\', (DI)
	MOVB BL, 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  check_vector

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

hex_escape:
	MOVB $'\\', (DI)
	MOVB $'u', 1(DI)
	MOVB $'0', 2(DI)
	MOVB $'0', 3(DI)

	MOVQ BX, R8
	SHRQ $4, R8
	MOVBQZX (R11)(R8*1), R10
	MOVB R10B, 4(DI)

	MOVQ BX, R9
	ANDQ $0x0f, R9
	MOVBQZX (R11)(R9*1), R10
	MOVB R10B, 5(DI)

	ADDQ $6, DI
	ADDQ $6, AX
	JMP  check_vector

escape_b:
	MOVB $'\\', (DI)
	MOVB $'b', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  check_vector

escape_f:
	MOVB $'\\', (DI)
	MOVB $'f', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  check_vector

escape_n:
	MOVB $'\\', (DI)
	MOVB $'n', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  check_vector

escape_r:
	MOVB $'\\', (DI)
	MOVB $'r', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  check_vector

escape_t:
	MOVB $'\\', (DI)
	MOVB $'t', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  check_vector

done:
	VZEROUPPER
	MOVQ AX, ret+24(FP)
	RET
