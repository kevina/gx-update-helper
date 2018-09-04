package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

type PackageFile struct {
	GxDependencies []PackageDep
	Name           string
	Gx             PackageGx
}

type PackageDep struct {
	Hash Hash
	Name string
}

type PackageGx struct {
	Dvcsimport string
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

type LastPubVer struct {
	Version string
	Hash    Hash
}

func ReadLastPubVer(dir string) (*LastPubVer, error) {
	str, err := ioutil.ReadFile(filepath.Join(dir, ".gx", "lastpubver"))
	if err != nil {
		return nil, err
	}
	str = bytes.TrimSpace(str)
	i := bytes.IndexByte(str, ':')
	if i == -1 || len(str) < i+1 || str[i+1] != ' ' {
		return nil, fmt.Errorf("bad lastpubver string")
	}
	return &LastPubVer{
		Version: string(str[:i-1]),
		Hash:    Hash(str[i+2:]),
	}, nil
}

func GetGxInfo() (pkg *PackageFile, lastPubVer *LastPubVer, err error) {
	pkg, err = ReadPackage(".")
	if err != nil {
		return
	}
	lastPubVer, err = ReadLastPubVer(".")
	return
}
