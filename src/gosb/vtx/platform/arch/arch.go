package arch

// FloatingPointData is a generic type, and will always be passed as a pointer.
// We rely on the individual arch implementations to meet all the necessary
// requirements. For example, on x86 the region must be 16-byte aligned and 512
// bytes in size.
type FloatingPointData byte
