package memview

import (
	"gosb/commons"
	"io/ioutil"
	"log"
	"runtime"
	"strconv"
	"strings"
)

var (
	Full       *commons.VMAreas = nil
	ASTemplate *AddressSpace    = nil

	// Due to concurrency issue, we might have delayed updates between
	// initialization of the full memory view, and setting up the hooks
	// in the runtime.
	EUpdates [50]*commons.VMArea
	CurrE    int = 0
	Updates  commons.VMAreas
)

func InitFullMemoryView() {
	// Allocate the emergency VMArea
	for i := range EUpdates {
		EUpdates[i] = &commons.VMArea{}
	}
	fvmas := ParseProcessAddressSpace(commons.USER_VAL)

	// Register the hook with the runtime.
	runtime.RegisterEmergencyGrowth(EmergencyGrowth)

	Full = commons.Convert(fvmas)

	// Generate the address space.
	ASTemplate = &AddressSpace{}
	//ASTemplate.Tables = pg.New(ASTemplate.PTEAllocator)
	ASTemplate.Initialize(Full)
}

//go:nosplit
func EmergencyGrowth(isheap bool, id int, start, size uintptr) {
	v := acquireUpdate()
	commons.Check(v != nil)
	v.Addr, v.Size, v.Prot = uint64(start), uint64(size), commons.HEAP_VAL
	Updates.AddBack(v.ToElem())
}

//go:nosplit
func acquireUpdate() *commons.VMArea {
	if CurrE < len(EUpdates) {
		i := CurrE
		CurrE++
		return EUpdates[i]
	}
	/*
		for i, v := range EUpdates {
			if v != nil {
				res := EUpdates[i]
				EUpdates[i] = nil
				return res
			}
		}*/
	return nil
}

// ParseProcessAddressSpace parses the self proc map to get the entire address space.
// defProt is the common set of flags we want for this.
func ParseProcessAddressSpace(defProt uint8) []*commons.VMArea {
	dat, err := ioutil.ReadFile("/proc/self/maps")
	if err != nil {
		log.Fatalf(err.Error())
	}
	tvmas := strings.Split(string(dat), "\n")
	vmareas := make([]*commons.VMArea, 0)
	for _, v := range tvmas {
		if len(v) == 0 || strings.Contains(v, "vsyscall") {
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
		rights := uint8(0)
		if strings.Contains(rstr, "r") {
			rights |= commons.R_VAL
		}
		// This doesn't work for some C dependencies that have ---p
		/*rights := uint8(commons.R_VAL)
		if !strings.Contains(rstr, "r") {
			log.Fatalf("missing read rights parsed from self proc: %v\n", rstr)
		}*/
		if strings.Contains(rstr, "w") {
			rights |= commons.W_VAL
		}
		if strings.Contains(rstr, "x") {
			rights |= commons.X_VAL
		}

		vm := &commons.VMArea{
			commons.ListElem{},
			commons.Section{
				Addr: uint64(start),
				Size: uint64(end - start),
				Prot: uint8(rights | defProt),
			},
		}
		vmareas = append(vmareas, vm)
	}
	return vmareas
}
