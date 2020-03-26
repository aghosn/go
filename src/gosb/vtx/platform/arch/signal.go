package arch

// SignalStack represents information about a user stack, and is equivalent to
// stack_t.
//
// +stateify savable
type SignalStack struct {
	Addr  uint64
	Flags uint32
	_     uint32
	Size  uint64
}
