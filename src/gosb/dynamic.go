package gosb

import (
	"errors"
	"fmt"
	"gosb/backend"
	"gosb/commons"
	"gosb/globals"
	"strings"
)

var (
	P2PDeps     map[string][]string
	PNames      map[string]bool
	IdToName    map[int]string
	SandboxDeps map[string][]string
)

func DynInitialize(back backend.Backend) {
	once.Do(func() {
		fmt.Println("Called init")
		globals.IdToPkg = make(map[int]*commons.Package)
		globals.NameToPkg = make(map[string]*commons.Package)
		globals.NameToId = make(map[string]int)
		globals.Sandboxes = make(map[commons.SandId]*commons.SandboxMemory)
		globals.Configurations = make([]*commons.SandboxDomain, 0)
		P2PDeps = make(map[string][]string)
		PNames = make(map[string]bool)
		IdToName = make(map[int]string)
		SandboxDeps = make(map[string][]string)
	})
}

//TODO modify this now.
func DynAddPackage(name string, id int, start, size uintptr) {
	commons.Check(globals.NameToPkg != nil)
	commons.Check(globals.IdToPkg != nil)
	commons.Check(globals.NameToId != nil)
	if _, ok := globals.IdToPkg[id]; ok {
		panic("Duplicated id for package")
	}
	n, ok := IdToName[id]
	if name == "module" && ok {
		name = n
	}
	pkg := &commons.Package{
		Name:  name,
		Id:    id,
		Sects: []commons.Section{commons.Section{uint64(start), uint64(size), 0}},
	}
	// Name should be updated in the register id.
	if name != "module" || ok {
		globals.AllPackages = append(globals.AllPackages, pkg)
		globals.NameToPkg[name] = pkg
	}
	globals.IdToPkg[id] = pkg
	//globals.NameToId[name] = id
}

func DynAddDependency(current, dependency string) {
	commons.Check(P2PDeps != nil)
	entry, _ := P2PDeps[current]
	P2PDeps[current] = append(entry, dependency)
	PNames[current] = true
	PNames[dependency] = true
}

func DynRegisterId(name string, id int) {
	PNames[name] = true
	pna, ok := IdToName[id]
	// If we are reusing an id, IdToName will be overwritten with the correct value.
	// Just have to make sure that it is not the one used by default in NameToId.
	if ok && pna != name {
		i, ok1 := globals.NameToId[pna]
		commons.Check(ok1 && id != i)
	}
	// Now it is safe to replace it in the map.
	IdToName[id] = name
	// Now set name to id, name could have mutiple ids, rely on IdToName to track.
	globals.NameToId[name] = id
	if pkg, ok := globals.IdToPkg[id]; ok {
		pkg.Name = name
		globals.AllPackages = append(globals.AllPackages, pkg)
		globals.NameToPkg[name] = pkg
	}
}

func DynRegisterSandbox(id, mem, sys string) {
	m, _, err := commons.ParseMemoryView(mem)
	s, err1 := commons.ParseSyscalls(sys)
	commons.Check(err == nil)
	commons.Check(err1 == nil)
	if e, ok := globals.Sandboxes[id]; ok {
		commons.Check(e.Config != nil)
		return
	}

	memview := make(map[string]uint8)
	for _, v := range m {
		memview[v.Name] = v.Perm
	}
	// Compute and translate the memview.
	//memview = globals.ComputeMemoryView(deps, P2PDeps, memview)
	// TODO compute the view.
	config := &commons.SandboxDomain{
		Id:       id,
		Func:     "a fake func",
		Sys:      s,
		View:     memview, //TODO complete the view
		Pkgs:     nil,     //TODO complete the pkgs
		Pristine: false,
	}
	globals.Configurations = append(globals.Configurations, config)

	sb := &commons.SandboxMemory{
		Static:  nil, //TODO Compute this.
		Config:  config,
		View:    nil,
		Entered: false,
	}
	globals.Sandboxes[id] = sb
}

func DynRegisterSandboxDependency(id, pkg string) {
	e, _ := SandboxDeps[id]
	SandboxDeps[id] = append(e, pkg)
}

func findId(name string) (int, error) {
	// Fast path
	if id, ok := globals.NameToId[name]; ok {
		return id, nil
	}

	// Slow path
	for k, v := range globals.NameToId {
		if strings.HasSuffix(k, fmt.Sprintf(".%s", name)) {
			return v, nil
		}
	}
	return -1, errors.New(fmt.Sprintf("Unable to find an id for %s", name))
}

func findFullName(pkg string) (string, error) {
	// Fast path
	if _, ok := globals.NameToId[pkg]; ok {
		return pkg, nil
	}

	// Slow path
	for k, _ := range globals.NameToId {
		if strings.HasSuffix(k, fmt.Sprintf(".%s", pkg)) {
			return k, nil
		}
	}

	// Couldn't find it inside the Id, look in deps
	for k, _ := range P2PDeps {
		if strings.HasSuffix(k, fmt.Sprintf(".%s", pkg)) {
			return k, nil
		}
	}
	return pkg, errors.New(fmt.Sprintf("Unable to find a full name for %s", pkg))
}

func DynProlog(id string) {
	sb, ok := globals.Sandboxes[id]
	commons.Check(ok)
	if sb.Entered {
		//TODO enter the sandbox
		return
	}

	// Compute full names for deps
	memview := sb.Config.View
	commons.Check(memview != nil)
	d, ok := SandboxDeps[id]
	commons.Check(ok)
	deps := make([]string, len(d))
	for i, v := range d {
		n, err := findFullName(v)
		commons.Check(err == nil)
		deps[i] = n
		if p, ok := memview[v]; ok {
			delete(memview, v)
			memview[n] = p
		}
	}

	// We need to update the view.
	sb.Config.View = globals.ComputeMemoryView(deps, P2PDeps, memview)
	mv := make(map[int]uint8)
	for k, v := range memview {
		i, _ := findId(k)
		mv[i] = v
	}
	commons.Check(sb.View == nil)
	sb.View = mv
	sb.Entered = true

	vmareas := make([]*commons.VMArea, 0)
	// Compute the static
	for k, v := range sb.Config.View {
		pkg, ok := globals.NameToPkg[k]
		if !ok {
			continue
		}
		//commons.Check(ok)
		vmareas = append(vmareas, commons.PackageToVMAreas(pkg, v)...)
	}
	sb.Static = commons.Convert(vmareas)
	fmt.Println("Prolog", id)
	sb.Static.Print()
}
