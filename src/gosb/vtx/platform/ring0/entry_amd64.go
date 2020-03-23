package ring0

// Start is the CPU entrypoint.
//
// The following start conditions must be satisfied:
//
//  * AX should contain the CPU pointer.
//  * c.GDT() should be loaded as the GDT.
//  * c.IDT() should be loaded as the IDT.
//  * c.CR0() should be the current CR0 value.
//  * c.CR3() should be set to the kernel PageTables.
//  * c.CR4() should be the current CR4 value.
//  * c.EFER() should be the current EFER value.
//
// The CPU state will be set to c.Registers().
//TODO(aghosn) implement
func Start() {}
