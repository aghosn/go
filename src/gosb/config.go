package gosb

import (
	"fmt"
	"strings"
)

// This file defines the format for sandbox configuration.
// These are the two strings passed to the sandbox, i.e., sandbox["main:R", "syscall"].\
//
// The first one represents the memory view, i.e., a refinement of the memory access rights
// over the default ones of this sandbox. By default, the sandbox get the original access rights to its
// code and data dependencies. This argument allows to further reduce these, or increase rights on packages
// that are not part of the sandbox dependencies (e.g., explicitely allow access to a pointer generated in
// another package).
// The grammar is:
// perm := [R]?[W]?[X]? || P
// entry := name:rights
// config := entry1,entry2,... // separated by commas
//
// The second argument represent syscall classes that are whitelisted for this sandbox.

type Entry struct {
	Name string
	Perm uint8
}

const (
	DELIMITER_PKGS  = ","
	DELIMITER_ENTRY = ":"

	// Permissions
	UNMAP    = "U"
	PRISTINE = "P"
	READ     = "R"
	WRITE    = "W"
	EXECUTE  = "X"
)

const (
	U_VAL = uint8(0)
	R_VAL = uint8(1)
	W_VAL = uint8(1 << 1)
	X_VAL = uint8(1 << 2)
	P_VAL = uint8(1 << 3)
)

func parseMemoryView(mem string) ([]Entry, error) {
	entries := strings.Split(mem, DELIMITER_PKGS)
	res := make([]Entry, len(entries))
	uniq := make(map[string]bool)
	for i, v := range entries {
		e, err := parseEntry(v)
		if err != nil {
			return res, err
		}
		if _, ok := uniq[e.Name]; ok {
			return nil, fmt.Errorf("Duplicated entry for %v\n", e.Name)
		}
		uniq[e.Name] = true
		res[i] = e
	}
	return res, nil
}

func parseEntry(entry string) (Entry, error) {
	split := strings.Split(entry, DELIMITER_ENTRY)
	if len(split) != 2 {
		return Entry{}, fmt.Errorf("Parsing error: expected 2 values, got %v\n", len(split))
	}
	name := split[0]
	if len(name) == 0 {
		return Entry{}, fmt.Errorf("Invalid package name of length 0\n")
	}
	perm, err := parsePerm(split[1])
	if err != nil {
		return Entry{}, err
	}
	return Entry{name, perm}, nil

}

func parsePerm(entry string) (uint8, error) {
	if len(entry) == 0 {
		return 0, fmt.Errorf("Unspecified permissions\n")
	}
	if len(entry) > 3 {
		return 0, fmt.Errorf("Invalid permission length %v\n", len(entry))
	}
	if entry == UNMAP {
		return U_VAL, nil
	}
	if entry == PRISTINE {
		return P_VAL, nil
	}
	perm := uint8(0)
	for i := 0; i < len(entry); i++ {
		char := string(entry[i])
		bit := uint8(0)
		switch char {
		case READ:
			bit = R_VAL
		case WRITE:
			bit = W_VAL
		case EXECUTE:
			bit = X_VAL
		default:
			return 0, fmt.Errorf("Invalid permission marker %v\n", char)
		}
		if (bit & perm) != 0 {
			return 0, fmt.Errorf("redundant permission marker %v in %v\n", char, entry)
		}
		perm |= bit
	}
	if (perm & R_VAL) == 0 {
		return 0, fmt.Errorf("Reading access right must be specified explicitly.\n")
	}
	return perm, nil
}
