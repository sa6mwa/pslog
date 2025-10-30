#include "textflag.h"

// func firstUnsafeIndexAsm(s string) int
TEXT ·firstUnsafeIndexAsm(SB), NOSPLIT, $0-24
	MOVD s_base+0(FP), R0    // pointer
	MOVD s_len+8(FP), R1     // remaining
	MOVD ZR, R2              // offset/result
	CBZ  R1, done

loop:
	MOVBU (R0), R3
	CMP   $0x80, R3
	BHS   safe
	CMP   $0x20, R3
	BLO   found
	CMP   $0x22, R3
	BEQ   found
	CMP   $0x5c, R3
	BEQ   found

safe:
	ADD   $1, R0
	ADD   $1, R2
	SUB   $1, R1
	CBNZ  R1, loop
	B     done

found:
	MOVD  R2, ret+16(FP)
	RET

done:
	MOVD  R2, ret+16(FP)
	RET
