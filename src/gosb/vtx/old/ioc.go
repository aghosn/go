package old

const (
	_IOC_NRBITS   = uintptr(8)
	_IOC_TYPEBITS = uintptr(8)
	// Defined as 13 in some places but redefined as 14 then,
	// see https://elixir.bootlin.com/linux/v4.20.17/source/include/uapi/asm-generic/ioctl.h#L32
	_IOC_SIZEBITS = uintptr(14)
	_IOC_DIRBITS  = uintptr(3)

	_IOC_NRMASK   = uintptr((1 << _IOC_NRBITS) - 1)
	_IOC_TYPEMASK = uintptr((1 << _IOC_TYPEBITS) - 1)
	_IOC_SIZEMASK = uintptr((1 << _IOC_SIZEBITS) - 1)
	_IOC_DIRMASK  = uintptr((1 << _IOC_DIRBITS) - 1)

	_IOC_NRSHIFT   = uintptr(0)
	_IOC_TYPESHIFT = uintptr(_IOC_NRSHIFT + _IOC_NRBITS)
	_IOC_SIZESHIFT = uintptr(_IOC_TYPESHIFT + _IOC_TYPEBITS)
	_IOC_DIRSHIFT  = uintptr(_IOC_SIZESHIFT + _IOC_SIZEBITS)

	// See values in https://elixir.bootlin.com/linux/v4.20.17/source/include/uapi/asm-generic/ioctl.h#L62
	_IOC_NONE  = uintptr(0)
	_IOC_WRITE = uintptr(1)
	_IOC_READ  = uintptr(2)
)

// These should be defined as in:
// https://elixir.bootlin.com/linux/v4.20.17/source/include/uapi/asm-generic/ioctl.h#L86
func _IO(tpe, nr uintptr) uintptr {
	return _IOC(_IOC_NONE, tpe, nr, 0)
}

func _IOR(tpe, nr, size uintptr) uintptr {
	return _IOC(_IOC_READ, tpe, nr, size)
}

func _IOW(tpe, nr, size uintptr) uintptr {
	return _IOC(_IOC_WRITE, tpe, nr, size)
}

func _IOWR(tpe, nr, size uintptr) uintptr {
	return _IOC(_IOC_READ|_IOC_WRITE, tpe, nr, size)
}

func _IOC(dir, tpe, nr, size uintptr) uintptr {
	return (((dir) << _IOC_DIRSHIFT) | ((tpe) << _IOC_TYPESHIFT) | ((nr) << _IOC_NRSHIFT) | ((size) << _IOC_SIZESHIFT))
}
