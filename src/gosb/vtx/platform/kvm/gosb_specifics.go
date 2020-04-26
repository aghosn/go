package kvm

import (
	"gosb/commons"
	"gosb/vtx/platform/vmas"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
)

//go:nosplit
func GetFs(addr *uint64)

// We need to be smart about allocations, try to stick to the vm as close as possible.
// Maybe we can change the allocation too.

func (m *Machine) setAllMemoryRegions() {
	// Set the memory allocator space
	for v := toArena(m.allocator.all.First); v != nil; v = toArena(v.Next) {
		m.setMemoryRegion(int(m.nextSlot), v.gpstart, _arenaPageSize, v.hvstart, 0)
		v.umemSlot = m.nextSlot
		m.nextSlot++
	}

	// Set the regular areas.
	areas := m.kernel.VMareas
	for v := vmas.ToVMA(areas.First); v != nil; v = vmas.ToVMA(v.Next) {
		m.setMemoryRegion(int(m.nextSlot), uintptr(v.PhysicalAddr), uintptr(v.Size), uintptr(v.Addr), 0)
		v.UmemSlot = m.nextSlot
		m.nextSlot++
	}
}

// ParseProcessAddressSpace parses the self proc map to get the entire address space.
// defProt is the common set of flags we want for this.
func ParseProcessAddressSpace(defProt uint8) []*vmas.VMArea {
	dat, err := ioutil.ReadFile("/proc/self/maps")
	if err != nil {
		log.Fatalf(err.Error())
	}
	tvmas := strings.Split(string(dat), "\n")
	vmareas := make([]*vmas.VMArea, 0)
	for _, v := range tvmas {
		if len(v) == 0 {
			continue
		}
		fields := strings.Fields(v)
		if len(fields) < 5 {
			log.Fatalf("error incomplete entry in /proc/self/maps: %v\n", fields)
		}
		// Parsing addresses.
		bounds := strings.Split(fields[0], "-")
		if len(bounds) != 2 {
			log.Fatalf("error founding bounds of area: %v\n", bounds)
		}
		start, err := strconv.ParseUint(bounds[0], 16, 64)
		end, err1 := strconv.ParseUint(bounds[1], 16, 64)
		if err != nil || err != nil {
			log.Fatalf("error parsing bounds of area: %v %v\n", err, err1)
		}
		// Parsing access rights.
		rstr := fields[1]
		rights := uint8(commons.R_VAL)
		if !strings.Contains(rstr, "r") {
			log.Fatalf("missing read rights parsed from self proc: %v\n", rstr)
		}
		if strings.Contains(rstr, "w") {
			rights |= commons.W_VAL
		}
		if strings.Contains(rstr, "x") {
			rights |= commons.X_VAL
		}

		vm := &vmas.VMArea{
			commons.ListElem{},
			commons.Section{
				Addr: uint64(start),
				Size: uint64(end - start),
				Prot: uint8(rights | defProt),
			},
			uintptr(start),
			^uint32(0),
		}
		vmareas = append(vmareas, vm)
	}
	return vmareas
}
