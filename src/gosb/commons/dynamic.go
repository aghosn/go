package commons

/* This is specific to the dynamic usage of LitterBox */

var (
	PythonRuntime = map[string]bool{
		"builtins": true,
		"sys":      true,
	}

	PythonFix = map[string]bool{
		"__main__": true,
	}
)
