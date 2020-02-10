package runtime

import ()

const (
	INCR_LVL   = 1
	CALLER_LVL = 2
	SKIPO1_LVL = 3
	MAX_LEVEL  = 4
)

func getpackageid(level int) int {
	if !bloatInitDone || level <= 0 || level > MAX_LEVEL {
		return -1
	}
	sp := getcallersp()
	pc := getcallerpc()
	gp := getg()
	var n int
	var pcbuf [MAX_LEVEL]uintptr
	systemstack(func() {
		n = gentraceback(pc, sp, 0, gp, 0, &pcbuf[0], level, nil, nil, 0)
	})

	if n != level {
		panic("Unable to unwind the stack")
	}
	f := findfunc(pcbuf[n-1])
	if !f.valid() {
		panic("Invalid function in stack unwind")
	}
	//println("The function name ", funcnameFromNameoff(f, f.nameoff))
	return pkgToId[nameToPkg(funcnameFromNameoff(f, f.nameoff))]
}

func filterPkgId(id int) int {
	if !bloatInitDone {
		if id == 0 {
			return 0
		}
		return -1
	}
	if _, ok := idToPkg[id]; ok {
		return id
	}
	return -1
}

func gosbInterpose(lvl int) int {
	id := -1
	mp := acquirem()
	if mp.tracingAlloc == 1 {
		goto cleanup
	}
	mp.tracingAlloc = 1
	id = filterPkgId(getpackageid(lvl + INCR_LVL))
cleanup:
	mp.tracingAlloc = 0
	releasem(mp)
	return id
}
