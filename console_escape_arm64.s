#include "textflag.h"

// func firstConsoleUnsafeIndexAsm(s string) int
TEXT ·firstConsoleUnsafeIndexAsm(SB), NOSPLIT, $0-24
	MOVD s_base+0(FP), R0    // pointer
	MOVD s_len+8(FP), R1     // remaining
	MOVD ZR, R2              // offset/result
	CBZ  R1, done

	MOVD $0x8080808080808080, R8  // asciiHighBitsMask
	MOVD $0x2020202020202020, R9  // jsonControlThreshold
	MOVD $0x0101010101010101, R10 // repeatOnes
	MOVD $0x2222222222222222, R11 // jsonQuoteMask
	MOVD $0x5c5c5c5c5c5c5c5c, R12 // jsonBackslashMask
	MOVD $0x2020202020202020, R13 // consoleSpaceMask
	MOVD $0x7f7f7f7f7f7f7f7f, R14 // consoleDelMask

chunk_loop:
	CMP  $16, R1
	BLT  chunk8

	MOVD (R0), R3
	AND  R8, R3, R4
	CBNZ R4, chunk_has_unsafe
	SUB  R9, R3, R4
	MVN  R3, R5
	AND  R4, R5, R4
	AND  R8, R4, R4
	CBNZ R4, chunk_has_unsafe
	EOR  R11, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe
	EOR  R12, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe
	EOR  R13, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe
	EOR  R14, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe

	MOVD 8(R0), R3
	AND  R8, R3, R4
	CBNZ R4, chunk_has_unsafe
	SUB  R9, R3, R4
	MVN  R3, R5
	AND  R4, R5, R4
	AND  R8, R4, R4
	CBNZ R4, chunk_has_unsafe
	EOR  R11, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe
	EOR  R12, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe
	EOR  R13, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe
	EOR  R14, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe

	ADD  $16, R0
	ADD  $16, R2
	SUB  $16, R1
	B    chunk_loop

chunk8:
	CMP  $8, R1
	BLT  tail_loop

	MOVD (R0), R3
	AND  R8, R3, R4
	CBNZ R4, chunk_has_unsafe
	SUB  R9, R3, R4
	MVN  R3, R5
	AND  R4, R5, R4
	AND  R8, R4, R4
	CBNZ R4, chunk_has_unsafe
	EOR  R11, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe
	EOR  R12, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe
	EOR  R13, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe
	EOR  R14, R3, R4
	SUB  R10, R4, R5
	MVN  R4, R6
	AND  R5, R6, R5
	AND  R8, R5, R5
	CBNZ R5, chunk_has_unsafe

	ADD  $8, R0
	ADD  $8, R2
	SUB  $8, R1
	B    chunk_loop

chunk_has_unsafe:
	// fallback to byte scan

 tail_loop:
	CBZ  R1, done

 tail_iter:
	MOVBU (R0), R3
	CMP   $0x80, R3
	BHS   found
	CMP   $0x20, R3
	BLS   found
	CMP   $0x22, R3
	BEQ   found
	CMP   $0x5c, R3
	BEQ   found
	CMP   $0x7f, R3
	BHS   found

	ADD   $1, R0
	ADD   $1, R2
	SUB   $1, R1
	CMP   $8, R1
	BGE   chunk8
	CBNZ  R1, tail_iter
	B     done

found:
	MOVD  R2, ret+16(FP)
	RET

done:
	MOVD  R2, ret+16(FP)
	RET
