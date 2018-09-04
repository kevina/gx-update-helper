package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var GOPATH string
var GXROOT string

func InitGlobal() error {
	GOPATH = os.Getenv("GOPATH")
	if GOPATH == "" {
		return fmt.Errorf("GOPATH not set")
	}
	GXROOT = filepath.Join(GOPATH, "src/gx/ipfs")
	return nil
}

func RootPath(path string) (string, error) {
	rootPath := filepath.Join(GOPATH, "src", path)
	curDir, err := os.Stat(".")
	if err != nil {
		return "", err
	}
	rootDir, err := os.Stat(rootPath)
	if err != nil {
		return "", err
	}
	if !os.SameFile(curDir, rootDir) {
		return "", fmt.Errorf("current directory not the projects root directory")
	}
	return rootPath, nil
}

func mainFun() error {
	err := InitGlobal()
	if err != nil {
		return err
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: %s rev-deps|init|next|published| ...", os.Args[0])
	}
	switch os.Args[1] {
	case "rev-deps":
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: %s rev-deps <name>", os.Args[0])
		}
		_, todoList, err := Gather(os.Args[2])
		if err != nil {
			return err
		}
		level := 0
		for _, todo := range todoList {
			if level != todo.Level {
				fmt.Printf("\n")
				level++
			}
			fmt.Printf("%s :: %s\n", todo.Path, strings.Join(todo.Deps, " "))
		}
	case "rev-deps-json":
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: %s rev-deps-json <name>", os.Args[0])
		}
		_, todoList, err := Gather(os.Args[2])
		if err != nil {
			return err
		}
		return todoList.ToJSON(os.Stdout)
	case "rev-deps-list", "bubble-list":
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: %s rev-deps-list <name>", os.Args[0])
		}
		_, todoList, err := Gather(os.Args[2])
		if err != nil {
			return err
		}
		for _, todo := range todoList {
			fmt.Printf("%s\n", todo.Path)
		}
	case "init":
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: %s init <name>", os.Args[0])
		}
		pkgs, todoList, err := Gather(os.Args[2])
		if err != nil {
			return err
		}
		// Make sure there are no duplicate entries
		_, err = todoList.CreateMap()
		if err != nil {
			return err
		}
		rootPath, err := RootPath(pkgs[""].Path)
		if err != nil {
			return err
		}
		path := filepath.Join(rootPath, ".mygx-workspace.json")
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
		if err != nil {
			return err
		}
		defer f.Close()
		err = todoList.ToJSON(f)
		if err != nil {
			return err
		}
		fmt.Printf("export MYGX_WORKSPACE=%s\n", path)
	case "status":
		todoList, todoByName, err := GetTodo()
		if err != nil {
			return err
		}
		level := 0
		for _, v := range todoList {
			if level != v.Level {
				fmt.Printf("\n")
				level++
			}
			if v.published {
				fmt.Printf("%s = %s\n", v.Path, v.NewHash)
				continue
			}
			extra := ""
			if len(v.NewDeps) > 0 {
				extra = " !!"
			}
			deps := []string{}
			for _, dep := range v.Deps {
				if !todoByName[dep].published {
					deps = append(deps, dep)
				}
			}
			if len(deps) == 0 {
				fmt.Printf("%s%s READY\n", v.Path, extra)
				continue
			}
			fmt.Printf("%s%s :: %s\n", v.Path, extra, strings.Join(deps, " "))
		}
	case "next":
		todoList, _, err := GetTodo()
		if err != nil {
			return err
		}
		for _, todo := range todoList {
			if !todo.published && todo.next {
				fmt.Printf("%s\n", todo.Path)
			}
		}
	case "published":
		todoList, todoByName, err := GetTodo()
		if err != nil {
			return err
		}
		pkg, lastPubVer, err := GetGxInfo()
		if err != nil {
			return err
		}
		todo, ok := todoByName[pkg.Name]
		if !ok {
			return fmt.Errorf("could not find entry for %s", pkg.Name)
		}
		todo.NewHash = lastPubVer.Hash
		depMap := map[string]Hash{}
		for _, dep := range pkg.GxDependencies {
			if todoByName[dep.Name] != nil {
				depMap[dep.Name] = dep.Hash
			}
		}
		todo.NewDeps = depMap
		err = todoList.Write()
		if err != nil {
			return err
		}		
	case "update-list":
		_, todoByName, err := GetTodo()
		if err != nil {
			return err
		}
		pkg, err := ReadPackage(".")
		if err != nil {
			return err
		}
		todo, ok := todoByName[pkg.Name]
		if !ok {
			return fmt.Errorf("could not find entry for %s", pkg.Name)
		}
		hashes := []Hash{}
		notReady := []string{}
		for _, dep := range todo.Deps {
			newHash := todoByName[dep].NewHash
			if newHash == "" {
				notReady = append(notReady, dep)
			} else {
				hashes = append(hashes, newHash)
			}
		}
		if len(notReady) > 0 {
			return fmt.Errorf("not yet updated: %s", strings.Join(notReady, " "))
		}
		for _, dep := range todo.AlsoUpdate {
			newHash := todoByName[dep].NewHash
			if newHash == "" {
				panic("inconsistent internal state")
			}
			hashes = append(hashes, newHash)
		}
		for _, hashes := range hashes {
			fmt.Printf("%s\n", hashes)
		}
	default:
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}
	return nil
}

func main() {
	err := mainFun()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
