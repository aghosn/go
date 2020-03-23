package pagetables

const (
	executeDisable = 1 << 63
	entriesPerPage = 512
)

// PTEs is a collection of entries.
type PTEs [entriesPerPage]PTE
