package runtime

func sandbox_prolog(mem string, syscalls string) {
	println("SB: prolog")
}

func sandbox_epilog(mem string, syscall string) {
	println("SB: epilog")
}
