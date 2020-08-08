package memview

import (
	c "gosb/commons"
	pg "gosb/vtx/platform/ring0/pagetables"
)

// Globals that are shared by everyone.
// These include:
// (1) A synchronized freespace allocator, used to map portions of the address
//	space that are above the 40bits limit. It is shared as VMs might have to
//	update each other.
// (2) God address space. This is a representation of the current program as
//	an address space that runtime routines can switch to without leaving the VM.
var (
	FreeSpace *FreeSpaceAllocator = nil
	GodAS     *AddressSpace       = nil
)

// Initialize creates a view of the entire address space and the GodAS.
// (1) parse the entire address space from self proc.
// (2) create the corresponding vmas.
// (3) mirror the full address space.
// (4) create a corresponding address space with the associated page tables.
func InitializeGod() {
	//TODO might have to handle emergency growth here.
	fvmas := ParseProcessAddressSpace(c.USER_VAL)
	full := c.Convert(fvmas)
	GodAS = &AddressSpace{}
	FreeSpace = &FreeSpaceAllocator{}

	// Create the free space allocator.
	free := full.Mirror()
	FreeSpace.Initialize(free, false)
	GodAS.FreeAllocator = FreeSpace

	// Create the page tables
	GodAS.PTEAllocator = &PageTableAllocator{}
	GodAS.PTEAllocator.Initialize(GodAS.FreeAllocator)
	GodAS.Tables = pg.New(GodAS.PTEAllocator)

	// Create the memory regions for GodAS
	for v := c.ToVMA(full.First); v != nil; {
		next := c.ToVMA(v.Next)
		full.Remove(v.ToElem())
		region := GodAS.VMAToMemoryRegion(v)
		GodAS.Regions.AddBack(region.ToElem())
		// update the loop
		v = next
	}
}
