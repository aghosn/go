package vtx

import (
//"gosb/commons"
//mv "gosb/vtx/platform/memview"
)

// mapAllArenas goes through each of the EMArenas and maps them
func mapAllArenas() {
	/*for _, m := range machines {
		m.Machine.MemView.PTEAllocator.All.Foreach(func(e *commons.ListElem) {
			for _, m1 := range machines {
				if m == m1 {
					continue
				}
				arena := mv.ToArena(e)
				other := &mv.OArena{A: arena}
				m1.Machine.MemView.PTEAllocator.AddOArena(other)
			}
		})
	}*/
	for _, m := range machines {
		m.Machine.MemView.MapArenas()
		//m.Machine.SetOArenaSlots()
	}
}
