package gc

import (
	"cmd/compile/internal/types"
)

type Pkg = types.Pkg

type PkgSet = map[*Pkg]bool

var sandboxes []*Node

var spoiledPackages PkgSet

func addPackages(packages []*Pkg) {
	for _, p := range packages {
		spoiledPackages[p] = true
	}
}

func registerSandboxes(top []*Node) {
	if spoiledPackages == nil {
		spoiledPackages = make(PkgSet)
	}
	for _, v := range top {
		if v.Op == ODCLFUNC && v.IsSandbox {
			sandboxes = append(sandboxes, v)
		}
	}
	for _, v := range sandboxes {
		pkgs := gatherPackages(v)
		addPackages(pkgs)
	}
}

func gatherPackages(n *Node) []*Pkg {
	var keys []*Pkg
	uniq := make(PkgSet)
	aggreg := make(map[*Node]*Pkg)
	gatherPackages1(n, aggreg)
	for _, p := range aggreg {
		if _, ok := uniq[p]; !ok && p != nil {
			uniq[p] = true
			keys = append(keys, p)
		}
	}
	return keys
}

// gatherPackages1 the aggreg is there to make sure we don't visit the same node
// too many times.
// This is a recursive function, that's why I think that we need the aggreg to
// avoid infinite loops. Let's see if that is the case. If not, I can simplify
// the function.
func gatherPackages1(n *Node, aggreg map[*Node]*Pkg) {
	if n == nil {
		return
	}
	if _, ok := aggreg[n]; ok {
		return
	}
	if p := getPackage(n); p != nil {
		aggreg[n] = p
	}
	gatherPackages1(n.Left, aggreg)
	gatherPackages1(n.Right, aggreg)
	gatherPackagesSlice(n.Ninit.Slice(), aggreg)
	gatherPackagesSlice(n.Nbody.Slice(), aggreg)
	gatherPackagesSlice(n.List.Slice(), aggreg)
	gatherPackagesSlice(n.Rlist.Slice(), aggreg)
}

func gatherPackagesSlice(nodes []*Node, aggreg map[*Node]*Pkg) {
	for _, v := range nodes {
		gatherPackages1(v, aggreg)
	}
}

// getPackage does its best to find a package for a given node.
func getPackage(n *Node) *Pkg {
	if n == nil {
		return nil
	}
	// We get the package directly from the symbol.
	if n.Sym != nil && n.Sym.Pkg != nil {
		return n.Sym.Pkg
	}
	// Special cases.
	switch n.Op {
	case ODCLFUNC:
		if n.Func == nil {
			panic("DCLFUNC has nil Func attribute.")
		}
		fname := n.Func.Nname
		if fname != nil && fname.Sym != nil && fname.Sym.Pkg != nil {
			return fname.Sym.Pkg
		}
		panic("DCLFUNC symbol without package.")
	case ONAME:
		name := n.Name
		if name == nil {
			panic("NAME node has nil name.")
		}
		if name.Pkg != nil {
			return name.Pkg
		}
		if p := getPackage(name.Pack); p != nil {
			return p
		}
		panic("NAME symbol without package.")
	}
	//TODO(aghosn) if we reach here, it means that we either failed,
	//or do not need to handle the node.
	return nil
}
