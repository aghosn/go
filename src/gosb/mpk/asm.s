#include "textflag.h"

TEXT ·WritePKRU(SB),$8
	MOVQ prot+0(FP), AX
	XORQ CX, CX
  XORQ DX, DX
  WRPKRU
	RET

TEXT ·ReadPKRU(SB),$8
	XORQ CX, CX
	RDPKRU
	MOVQ AX, ret+8(FP)
	RET
