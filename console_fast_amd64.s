#include "textflag.h"

// func firstConsoleUnsafeIndexSSE(s string) int
TEXT ·firstConsoleUnsafeIndexSSE(SB), NOSPLIT, $0-24
	MOVQ s_base+0(FP), SI
	MOVQ s_len+8(FP), CX
	XORQ AX, AX
	TESTQ CX, CX
	JE   done

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
