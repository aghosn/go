package runtime

func sandbox_prolog(mem string, syscalls string) {
	println("SB: prolog", mem, syscalls)
}

func sandbox_epilog(mem string, syscalls string) {
	println("SB: epilog", mem, syscalls)
}
