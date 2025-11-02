#include "textflag.h"

// func Read() uint64
TEXT ·Read(SB), NOSPLIT, $0-8
    LFENCE
    RDTSC
    SHLQ $32, DX
    ORQ AX, DX
    MOVQ DX, ret+0(FP)
    RET
