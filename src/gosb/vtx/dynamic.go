package vtx

import (
	"gosb/commons"
	"gosb/globals"
	"gosb/vtx/platform/kvm"
	mv "gosb/vtx/platform/memview"
	"os"
	"runtime"
	"sync"
	"syscall"
)

/* Implementation of the dynamic version of the vtx backend.
* We have some weird requirements due to the dynamicity of python that we try to
* account for here.
* */

var (
	ionce  sync.Once
	inside bool = false
)

func DInit() {
	// Delay the initialization to the first prolog.
}

func internalInit() {
	ionce.Do(func() {
		kvm.KVMInit()
		mv.InitializeGod()

		// Skip the init of sandboxes as we probably don't have them.
		views = make(map[commons.SandId]*mv.AddressSpace)

		// Initialize the kvm state.
		var err error
		kvmFd, err = os.OpenFile(_KVM_DRIVER_PATH, syscall.O_RDWR, 0)
		commons.Check(err == nil)
		err = kvm.UpdateGlobalOnce(int(kvmFd.Fd()))
		commons.Check(err == nil)
		machine = kvm.CreateVirtualMachine(int(kvmFd.Fd()), false)
		vm = &kvm.KVM{machine, nil, "God", 0}

		// Map the page allocator.
		mv.GodAS.MapArenas(false)
	})
}

func DProlog(id commons.SandId) {
	internalInit()
	commons.Check(views != nil)
	sb, ok := globals.Sandboxes[id]
	commons.Check(ok)
	mem, ok := views[id]
	if ok {
		goto entering
	}

	// We need to create the sandbox.
	commons.Check(mv.GodAS != nil)
	dynTryInHost(func() {
		mem = mv.GodAS.Copy(false)
		mem.ApplyDomain(sb)
		disablePkgs(mem, sb)
		views[sb.Config.Id] = mem
		machine.UpdateEPTSlots(func(start, size, gpa uintptr) {
			mv.GodAS.DefaultMap(start, size, gpa)
			for _, v := range views {
				v.DefaultMap(start, size, gpa)
			}
		})
	})
entering:
	dprolog(sb, mem)
}

//go:nosplit
func dprolog(sb *commons.SandboxMemory, mem *mv.AddressSpace) {
	commons.Check(mv.GodAS != nil)
	if !inside {
		prolog_internal(true)
		inside = true
	}
	vcpu := runtime.GetVcpu()
	kvm.RedSwitch(uintptr(mem.Tables.CR3(false, 0)))
	kvm.SetVCPUAttributes(vcpu /*mv.GodAS*/, mem, &sb.Config.Sys)
}

//go:nosplit
func DEpilog(id commons.SandId) {
	vcpu := runtime.GetVcpu()
	commons.Check(inside)
	commons.Check(vcpu != 0)
	commons.Check(globals.DynGetPrevId != nil)
	// For the moment disallow nested sandboxes.
	commons.Check(globals.DynGetPrevId() == "GOD")
	kvm.Redpill(kvm.RED_GOD)
	kvm.SetVCPUAttributes(vcpu, mv.GodAS, &commons.SyscallAll)

}

//go:nosplit
func DynTransfer(oldid, newid int, start, size uintptr) {
	throw("Now we here")
}

func DRuntimeGrowth(isheap bool, id int, start, size uintptr) {
	if mv.GodAS == nil {
		return
	}
	size = uintptr(commons.Round(uint64(size), true))
	mem := &mv.MemoryRegion{} //mv.GodAS.AcquireEMR()
	mv.GodAS.Extend(false, mem, uint64(start), uint64(size), commons.HEAP_VAL)
	//TODO is that correct actually? Shouldn't it be mapped in the same way?
	for _, v := range views {
		v.Extend(false, mem, uint64(start), uint64(size), commons.HEAP_VAL)
	}
}

/* Helper functions */

//go:nosplit
func dynTryInHost(f func()) {
	commons.Check(globals.DynGetId != nil)
	if !inside {
		f()
		return
	}
	// We are inside
	kvm.Redpill(kvm.RED_EXIT)
	runtime.AssignVcpu(0)
	f()
	prolog_internal(false)
	inside = true
}

// disablePkgs removes all the pkgs that should not be available.
func disablePkgs(mem *mv.AddressSpace, sb *commons.SandboxMemory) {
	for _, pkg := range globals.AllPackages {
		// Supposed to be there, leave it.
		if _, ok := sb.Config.View[pkg.Name]; ok {
			continue
		}
		i, err := globals.DynFindId(pkg.Name)
		if _, ok := sb.View[i]; ok && err == nil {
			continue
		}
		// Not supposed to be mapped.
		for _, s := range pkg.Sects {
			mem.ToggleDyn(false, uintptr(s.Addr), uintptr(s.Size), commons.UNMAP_VAL)
		}
	}
}
