package memview

import (
	"gosb/commons"
	"gosb/vmas"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
)

var (
	FullAddressSpace     *vmas.VMAreas = nil
	AddressSpaceTemplate *AddressSpace = nil
)

func InitFullMemoryView() {
	fvmas := ParseProcessAddressSpace(commons.USER_VAL)
	FullAddressSpace = vmas.Convert(fvmas)

	// Generate the address space.
	AddressSpaceTemplate = &AddressSpace{}
	AddressSpaceTemplate.Initialize(FullAddressSpace)
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

		vm := &vmas.VMArea{
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
