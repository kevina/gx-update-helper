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
		todoList, err := ReadTodo()
		if err != nil {
			return err
		}
		todoByName, err := todoList.CreateMap()
		if err != nil {
			return err
		}
		level := 0
		for _, v := range todoList {
			if level != v.Level {
				fmt.Printf("\n")
				level++
			}
			if v.NewHash != "" {
				fmt.Printf("%s = %s\n", v.Path, v.NewHash)
				continue
			}
			deps := []string{}
			for _, dep := range v.Deps {
				if todoByName[dep].NewHash == "" {
					deps = append(deps, dep)
				}
			}
			if len(deps) == 0 {
				fmt.Printf("%s READY\n", v.Path)
				continue
			}
			fmt.Printf("%s :: %s\n", v.Path, strings.Join(deps, " "))
		}
	case "next":
		todoList, err := ReadTodo()
		if err != nil {
			return err
		}
		todoByName, err := todoList.CreateMap()
		if err != nil {
			return err
		}
		for _, v := range todoList {
			if v.NewHash != "" {
				continue
			}
			ok := true
			for _, dep := range v.Deps {
				if todoByName[dep].NewHash == "" {
					ok = false
				}
			}
			if ok {
				fmt.Printf("%s\n", v.Path)
			}
		}
	case "sync":
		todoList, err := ReadTodo()
		if err != nil {
			return err
		}
		todoByName, err := todoList.CreateMap()
		if err != nil {
			return err
		}
		pkg, err := ReadPackage(".")
		if err != nil {
			return err
		}
		lastPubVer, err := ReadLastPubVer(".")
		if err != nil {
			return err
		}
		v, ok := todoByName[pkg.Name]
		if !ok {
			return fmt.Errorf("could not find entry for %s", pkg.Name)
		}
		if v.NewHash == lastPubVer.Hash {
			fmt.Fprintf(os.Stderr, "nothing to change: already marked as published with hash %s\n", v.NewHash)
			return nil
		} else if v.OrigHash == lastPubVer.Hash {
			if v.NewHash == "" {
				fmt.Fprintf(os.Stderr, "nothing to change: not yet published with new hash %s\n", v.NewHash)
				return nil
			} else {
				v.NewHash = ""
				fmt.Fprintf(os.Stderr, "resetting state to unpublished\n")
			}
		} else {
			if v.NewHash != "" {
				fmt.Fprintf(os.Stderr, "warning: previous published using hash %s\n", v.NewHash)
			}
			v.NewHash = lastPubVer.Hash
			fmt.Fprintf(os.Stderr, "marking state as published with hash %s\n", v.NewHash)
		}
		err = todoList.Write()
		if err != nil {
			return err
		}
	case "update-cmds":
		todoList, err := ReadTodo()
		if err != nil {
			return err
		}
		todoByName, err := todoList.CreateMap()
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
		cmds := []string{}
		toUpdate := []string{}
		for _, dep := range todo.Deps {
			newHash := todoByName[dep].NewHash
			if newHash == "" {
				toUpdate = append(toUpdate, dep)
			} else {
				cmds = append(cmds, fmt.Sprintf("gx update %s %s\n", dep, newHash))
			}
		}
		if len(toUpdate) > 0 {
			return fmt.Errorf("not yet updated: %s", strings.Join(toUpdate, " "))
		}
		for _, dep := range todo.AlsoUpdate {
			newHash := todoByName[dep].NewHash
			if newHash == "" {
				panic("inconsistent internal state")
			}
			cmds = append(cmds, fmt.Sprintf("gx update %s %s\n", dep, newHash))
		}
		for _, cmd := range cmds {
			os.Stdout.WriteString(cmd)
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
