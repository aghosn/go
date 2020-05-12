package sim

import (
	c "gosb/commons"
	//	g "gosb/globals"
	//	"log"
)

var (
	countEntries map[c.SandId]int
)

//go:noinline
//go:nosplit
func Init() {
	/*log.Println("Init simulation backend.")
	countEntries = make(map[c.SandId]int)
	for _, d := range g.Sandboxes {
		countEntries[d.Config.Id] = 0
	}*/
}

//go:noinline
//go:nosplit
func Prolog(id c.SandId) {
	/*	if _, ok := g.Sandboxes[id]; ok {
			log.Printf("Prolog sandbox %v\n", id)
			count, _ := countEntries[id]
			countEntries[id] = count + 1
			return
		}
		// Error for the sandboxes.
		for _, s := range g.Sandboxes {
			log.Printf("sandbox: %v\n", s.Config.Id)
		}

		log.Fatalf("Error prolog: unable to resolve sandbox %v\n", id)*/
}

//go:noinline
//go:nosplit
func Epilog(id c.SandId) {
	/*if _, ok := g.Sandboxes[id]; ok {
		log.Printf("Epilog sandbox %v\n", id)
		count, _ := countEntries[id]
		countEntries[id] = count - 1
		if count-1 < 0 {
			log.Fatalf("Error: sandbox %v has negative entry count\n", id)
		}
		return
	}
	log.Fatalf("Error epilog: unable to resolve sandbox %v\n", id)*/
}

//go:nosplit
func Transfer(oldid, newid int, start, size uintptr) {
	// Cannot do anything for now because of malloc
	return
	// See which sandboxes need to be updated to remove access.
	/*if sbs, ok := g.PkgIdToSid[oldid]; ok {
		log.Printf("Removing %d:[%x-%x] from: ", oldid, start, start+size)
		for _, sb := range sbs {
			log.Printf("'%v '", sb)
			sand, ok1 := g.Domains[sb]
			if !ok1 {
				log.Fatalf("Error: unable to access sandbox %v\n", sb)
			}
			found := false
			for _, pkg := range sand.SPkgs {
				if pkg.Id == oldid {
					found = true
					break
				}
			}
			if !found {
				log.Fatalf("Error: package %v is not part of sb %v\n", oldid, sb)
			}
		}
	} else {
		log.Printf("No removal needed for pkg %v\n", oldid)
	}

	// Now add to the ones that need it.
	if sbs, ok := g.PkgIdToSid[newid]; ok {
		log.Printf("Adding %d:[%x-%x] from: ", newid, start, start+size)
		for _, sb := range sbs {
			log.Printf("'%v '", sb)
			sand, ok1 := g.Domains[sb]
			if !ok1 {
				log.Fatalf("Error: unable to access sandbox %v\n", sb)
			}
			found := false
			for _, pkg := range sand.SPkgs {
				if pkg.Id == newid {
					found = true
					break
				}
			}
			if !found {
				log.Fatalf("Error: package %v is not part of sb %v\n", newid, sb)
			}
		}
	} else {
		log.Print("No adding needed for pkg %v\n", newid)
	}*/
}

//go:nosplit
func Register(id int, start, size uintptr) {
	// Cannot do anything now because of malloc
	return
	/*	log.Printf("Registering %d[%x-%x]\n", id, start, start+size)
		// Now add to the ones that need it.
		if sbs, ok := g.PkgIdToSid[id]; ok {
			log.Printf("Adding %d:[%x-%x] from: ", id, start, start+size)
			for _, sb := range sbs {
				log.Printf("'%v '", sb)
				sand, ok1 := g.Domains[sb]
				if !ok1 {
					log.Fatalf("Error: unable to access sandbox %v\n", sb)
				}
				found := false
				for _, pkg := range sand.SPkgs {
					if pkg.Id == id {
						found = true
						break
					}
				}
				if !found {
					log.Fatalf("Error: package %v is not part of sb %v\n", id, sb)
				}
			}
		} else {
			log.Print("No adding needed for pkg %v\n", id)
		}*/
}

//go:nosplit
func Execute(id c.SandId) {
	return
	/*if _, ok := g.Domains[id]; ok {
		log.Printf("Execute sandbox %v\n", id)
		return
	}
	log.Fatalf("Error: unable to find sandbox %v\n", id)*/
}
