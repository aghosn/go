package runtime

func sandbox_prolog(mem string, syscalls string) {
	panic("I'm in the prolog")
}

func sandbox_epilog(mem string, syscall string) {
	panic("I'm in the epilog")
}
