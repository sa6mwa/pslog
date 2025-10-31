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

loop:
	CMPQ CX, $0
	JE   done

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
copy_safe:
	MOVB BL, (DI)
	INCQ DI
	INCQ AX
	JMP  loop

quote_escape:
	MOVB $'\\', (DI)
	MOVB BL, 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  loop

backslash_escape:
	MOVB $'\\', (DI)
	MOVB BL, 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  loop

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
	JMP  loop

escape_f:
	MOVB $'\\', (DI)
	MOVB $'f', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  loop

escape_n:
	MOVB $'\\', (DI)
	MOVB $'n', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  loop

escape_r:
	MOVB $'\\', (DI)
	MOVB $'r', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  loop

escape_t:
	MOVB $'\\', (DI)
	MOVB $'t', 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  loop

hex_escape:
	MOVB $'\\', (DI)
	MOVB $'u', 1(DI)
	MOVB $'0', 2(DI)
	MOVB $'0', 3(DI)

	MOVBQZX -1(SI), R8
	MOVQ R8, R9
	SHRQ $4, R8
	MOVBQZX (R11)(R8*1), R10
	MOVB R10B, 4(DI)

	ANDQ $0x0f, R9
	MOVBQZX (R11)(R9*1), R10
	MOVB R10B, 5(DI)

	ADDQ $6, DI
	ADDQ $6, AX
	JMP  loop

done:
	MOVQ AX, ret+24(FP)
	RET
