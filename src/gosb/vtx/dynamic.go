package vtx

import (
	"fmt"
	"gosb/commons"
	"gosb/globals"
	"gosb/vtx/platform/kvm"
	mv "gosb/vtx/platform/memview"
	"os"
	"runtime"
	"syscall"
)

/* Implementation of the dynamic version of the vtx backend.
* We have some weird requirements due to the dynamicity of python that we try to
* account for here.
* */

var (
	inside bool = false
)

// TODO we need to init
// We need to register sandbox
// We need to have the prolog with a hook for the current sandbox id
// We need to have epilog
// We need to register growth.

func DInit() {
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
}

func DProlog(id commons.SandId) {
	commons.Check(views != nil)
	sb, ok := globals.Sandboxes[id]
	commons.Check(ok)
	mem, ok := views[id]
	if ok {
		goto entering
	}

	// We need to create the sandbox.
	commons.Check(mv.GodAS != nil)
	mem = mv.GodAS.Copy(false)
	mem.ApplyDomain(sb)
	views[sb.Config.Id] = mem
entering:
	dprolog(sb, mem)
}

//go:nosplit
func dprolog(sb *commons.SandboxMemory, mem *mv.AddressSpace) {
	if !inside {
		fmt.Printf("About to switch to %x\n", mv.GodAS.Tables.CR3(false, 0))
		prolog_internal(true)
		inside = true
	}
	vcpu := runtime.GetVcpu()
	//kvm.RedSwitch(uintptr(mem.Tables.CR3(false, 0)))
	kvm.SetVCPUAttributes(vcpu, mem, &sb.Config.Sys)
}

//go:nosplit
func DynTransfer(oldid, newid int, start, size uintptr) {
	throw("Now we here")
}

func DRuntimeGrowth(isheap bool, id int, start, size uintptr) {
	size = uintptr(commons.Round(uint64(size), true))
	mem := &mv.MemoryRegion{} //mv.GodAS.AcquireEMR()
	mv.GodAS.Extend(false, mem, uint64(start), uint64(size), commons.HEAP_VAL)
}
