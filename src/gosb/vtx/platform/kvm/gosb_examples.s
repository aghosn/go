
#include "textflag.h"

TEXT ·Mine2(SB),NOSPLIT,$0
	PUSHQ AX
	CALL ·Mine(SB) 
	POPQ AX
	RET

TEXT ·Bluepillret(SB),NOSPLIT,$0
	//PUSHQ $555
	//BYTE $0x9C
	//CALL ·Mine(SB)
	RET



//TEXT ·Mine(SB),NOSPLIT,$0
//	PUSHQ $555
//	BYTE $0xCC	
//	POPQ AX
//	RET
//	MOVQ $555, R11
//	PUSHQ $666
//	BYTE $0x9c
//	POPQ R10
//	BYTE $0xCC
//	RET
