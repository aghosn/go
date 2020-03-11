TEXT ·WritePKRU(SB),$0
	MOVQ prot+0(FP), AX
	XORQ CX, CX
    XORQ DX, DX
    WRPKRU
	RET

TEXT ·ReadPKRU(SB),$0
	XORQ CX, CX
	RDPKRU
	MOVQ AX, ret+8(FP)
	RET
