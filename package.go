package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
)

type PackageFile struct {
	GxDependencies []PackageDep `json:"gxDependencies"`
	Name           string       `json:"name"`
}

type PackageDep struct {
	Hash Hash   `json:"hash"`
	Name string `json:"name"`
}

func GxDir(hash Hash, name string) string {
	return filepath.Join(GXROOT, string(hash), name)
}

func ReadPackage(dir string) (*PackageFile, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, err
	}
	pkg := &PackageFile{}
	err = json.Unmarshal(bytes, pkg)
	if err != nil {
		return nil, err
	}
	return pkg, nil
}

