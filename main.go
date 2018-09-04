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
	return path, nil
}

func mainFun() error {
	err := InitGlobal()
	if err != nil {
		return err
	}
	if len(os.Args) != 3 {
		return fmt.Errorf("usage: %s rev-deps|rev-deps-json|rev-deps-list <name>", os.Args[0])
	}
	pkgs, todoList, err := Gather(os.Args[2])
	if err != nil {
		return err
	}
	switch os.Args[1] {
	case "rev-deps":
		level := 0
		for _, todo := range todoList {
			if level != todo.Level {
				fmt.Printf("\n")
				level++
			}
			fmt.Printf("%s :: %s\n", todo.Name, strings.Join(todo.Deps, " "))
		}
	case "rev-deps-json":
		return todoList.Write(os.Stdout)
	case "rev-deps-list":
		for _, todo := range todoList {
			fmt.Printf("%s\n", todo.Path)
		}
	case "init":
		rootPath, err := RootPath(pkgs[""].Path)
		if err != nil {
			return err
		}
		path := filepath.Join(rootPath, ".mygx-workspace.json")
		todoList.WriteToFile(path)
		if err != nil {
			return err
		}
		fmt.Printf("MYGX_WORKSPACE=%s\n", path)
	case "next":
		todoList, err := ReadTodo()
		if err != nil {
			return err
		}
		todoByName := map[string]*Todo{}
		for _, v := range todoList {
			todoByName[v.Name] = v
		}
		for _, v := range todoList {
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
