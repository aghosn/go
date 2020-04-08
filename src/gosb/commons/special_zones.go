package commons

const (
	// reservedMemory is a chunk of physical memory reserved starting at
	// physical address zero. There are some special pages in this region,
	// so we just call the whole thing off.
	ReservedMemory = 0x100000000
)

// Extra symbols we need to locate
var (
	ExtraSymbols = map[string]uint8{"runtime.types": R_VAL}
)
