package pagetables

// PageTables is a set of page tables.
type PageTables struct {
	// Allocator is used to allocate nodes.
	Allocator Allocator

	// root is the pagetable root.
	root *PTEs

	// rootPhysical is the cached physical address of the root.
	//
	// This is saved only to prevent constant translation.
	rootPhysical uintptr
}
