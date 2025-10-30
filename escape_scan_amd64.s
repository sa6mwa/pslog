#include "textflag.h"

// func firstUnsafeIndexSSE(s string) int
TEXT ·firstUnsafeIndexSSE(SB), NOSPLIT, $0-24
	MOVQ s_base+0(FP), SI      // pointer to data
	MOVQ s_len+8(FP), CX       // remaining length
	XORQ AX, AX                // offset/result
	TESTQ CX, CX
	JEQ  done

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
	// fallthrough into tail loop to inspect byte by byte

tail_loop:
	TESTQ CX, CX
	JEQ   done

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
