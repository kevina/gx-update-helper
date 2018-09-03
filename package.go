package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
)

type packageFile struct {
	GxDependencies []packageDep
	Name           string
	Gx             packageGx
}

type packageDep struct {
	Hash Hash
	Name string
}

type packageGx struct {
	Dvcsimport string
}

func GxDir(hash Hash, name string) string {
	return filepath.Join(GXROOT, string(hash), name)
}

func ReadPackage(dir string) (*packageFile, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, err
	}
	pkg := &packageFile{}
	err = json.Unmarshal(bytes, pkg)
	if err != nil {
		return nil, err
	}
	return pkg, nil
}
