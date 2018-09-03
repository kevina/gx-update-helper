package main

import (
	"fmt"
	"sort"
)

func (pkgs Packages) ByName(name string) *PkgInfo {
	for _, pkg := range pkgs {
		if pkg.Name == name {
			return pkg
		}
	}
	return nil
}

func (pkgs Packages) Names(hashes []Hash) []string {
	names := make([]string, len(hashes))
	for i, hash := range hashes {
		names[i] = pkgs[hash].Name
	}
	sort.Strings(names)
	return names
}

func (pkgs Packages) Dump() {
	for hash, pkg := range pkgs {
		fmt.Printf("%s %s ::\n", hash, pkg.Name)
		for dephash, dep := range pkg.Deps {
			fmt.Printf("  %s %s \n", dephash, dep.Name)
		}
		fmt.Printf("\n")
	}
}

func GatherDeps(pkgs Packages, root Hash, pkgDir string) (*PkgInfo, error) {
	if pkgs[root] != nil {
		return pkgs[root], nil // already processed
	}
	jsonPkg, err := ReadPackage(pkgDir)
	if err != nil {
		return nil, err
	}
	pkg := &PkgInfo{
		Hash:       root,
		Name:       jsonPkg.Name,
		Deps:       Packages{},
		DirectDeps: Packages{},
	}
	for _, dep := range jsonPkg.GxDependencies {
		depPkg, err := GatherDeps(pkgs, dep.Hash, GxDir(dep.Hash, dep.Name))
		if err != nil {
			return nil, err
		}
		pkg.Deps[dep.Hash] = depPkg
		pkg.DirectDeps[dep.Hash] = depPkg
		// collect any extra dependencies (performs closure)
		for _, subdep := range depPkg.Deps {
			pkg.Deps[subdep.Hash] = subdep
		}
	}
	pkgs[root] = pkg
	return pkg, nil
}

func (pkgs Packages) RevDeps(hash Hash) DepSet {
	revDeps := DepSet{}
	for dephash, dep := range pkgs {
		if dep.Deps[hash] != nil {
			revDeps.Add(dephash)
		}
	}
	return revDeps
}

func (pkgs Packages) Intersect(deps DepSet) DepSet {
	intersect := DepSet{}
	for dephash, _ := range pkgs {
		if deps.Has(dephash) {
			intersect.Add(dephash)
		}
	}
	return intersect
}

type RevDep struct {
	Hash         Hash
	Level        int
	DirectDeps   []Hash // Deps to update that depend on previous level
	AlsoUpdate   []Hash // Deps that also need to be updated
	IndirectDeps []Hash //
}

func BubbleList(pkgs Packages, hash Hash) []RevDep {
	// Start by getting the rev deps for the hash
	lst := []RevDep{}
	deps := pkgs.RevDeps(hash)
	deps.Add(hash)
	// Now determine which of those packages depends on each other
	depMap := map[Hash]DepSet{}
	fullDeps := map[Hash]DepSet{}
	for dephash, _ := range deps {
		depMap[dephash] = pkgs[dephash].Deps.Intersect(deps)
		fullDeps[dephash] = depMap[dephash].Clone()
	}
	level := 0
	iterate := func(directDeps ...Hash) []Hash {
		res := []Hash{}
		for dephash, deps := range depMap {
			pruned := []Hash{}
			for _, toDel := range directDeps {
				if deps.Has(toDel) {
					deps.Del(toDel)
					pruned = append(pruned, toDel)
				}
			}
			if deps.Len() > 0 {
				continue
			}
			if len(directDeps) > 0 && len(pruned) == 0 {
				panic("pruned list empty")
			}
			delete(depMap, dephash)

			toUpdate := pkgs[dephash].DirectDeps.Intersect(fullDeps[dephash])
			indirect := fullDeps[dephash]

			delete(fullDeps, dephash)
			indirect.Del(pruned...)
			c := toUpdate.Del(pruned...)
			if c != len(pruned) {
				panic("direct deps not in package.json")
			}

			res = append(res, dephash)

			lst = append(lst, RevDep{
				Hash:         dephash,
				Level:        level,
				DirectDeps:   pruned,
				AlsoUpdate:   toUpdate.Elms(),
				IndirectDeps: indirect.Elms(),
			})
		}
		level += 1
		return res
	}
	next := iterate()
	for len(next) > 0 {
		next = iterate(next...)
	}
	return lst
}
