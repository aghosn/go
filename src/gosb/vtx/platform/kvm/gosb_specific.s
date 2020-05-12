#include "textflag.h"

#define SYS_arch_prctl		158

TEXT ·GetFs(SB),NOSPLIT,$32
	MOVQ addr+0(FP), SI
	MOVQ $0x1003, DI // ARCH_GET_FS
	MOVQ $SYS_arch_prctl, AX
	SYSCALL
	CMPQ AX, $0xfffffffffffff001
	JLS 2(PC)
	MOVL $0xf1, 0xf1
	RET


