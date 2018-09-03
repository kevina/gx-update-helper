package main

type Hash string

type PkgInfo struct {
	Hash       Hash
	Name       string
	DirectDeps Packages
	Deps       Packages // transitive closure of all deps
}

type Packages map[Hash]*PkgInfo

