package runtime

const (
	INCR_LVL   = 1
	CALLER_LVL = 2
	SKIPO1_LVL = 3
	MAX_LEVEL  = 4
)

// Exposing runtime locks to the outside.
type GosbMutex struct {
	m mutex
}

func (g *GosbMutex) Lock() {
	lock(&g.m)
}

func (g *GosbMutex) Unlock() {
	unlock(&g.m)
}

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
	id := pcToPkg(pcbuf[n-1])
	if id != 0 && gp.pristine {
		return gp.pristineid
	}
	return id
}

func filterPkgId(id int) int {
	if !bloatInitDone {
		if !mainInitDone || id == 0 {
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
	if !bloatInitDone {
		return 0
	}
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
