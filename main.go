package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

type Todo struct {
	Name       string
	Level      int
	NewHash    Hash     `json:",omitempty"`
	NewVersion string   `json:",omitempty"`
	OrigHash   Hash     `json:",omitempty"`
	Deps       []string `json:",omitempty"`
	AlsoUpdate []string `json:",omitempty"`
	Indirect   []string `json:",omitempty"`
}

func (x *Todo) Less(y *Todo) bool {
	if x.Level != y.Level {
		return x.Level < y.Level
	}
	if len(x.Deps) != len(y.Deps) {
		return len(x.Deps) < len(y.Deps)
	}
	for i := 0; i < len(x.Deps); i++ {
		if x.Deps[i] != y.Deps[i] {
			return x.Deps[i] < y.Deps[i]
		}
	}
	return x.Name < y.Name
}

func mainFun() error {
	err := InitGlobal()
	if err != nil {
		return err
	}
	if len(os.Args) != 3 || os.Args[1] != "rev-deps" {
		return fmt.Errorf("usage: %s rev-deps <name>", os.Args[0])
	}
	pkgs := Packages{}
	_, err = GatherDeps(pkgs, "", ".")
	if err != nil {
		return fmt.Errorf("could not gather deps: %s", err.Error())
	}
	//pkgs.Dump()
	target := pkgs.ByName(os.Args[2])
	if target == nil {
		return fmt.Errorf("package not found: %s", os.Args[2])
	}
	lst := BubbleList(pkgs, target.Hash)
	todoList := []*Todo{}
	for _, dep := range lst {
		todoList = append(todoList, &Todo{
			Name:       pkgs[dep.Hash].Name,
			Level:      dep.Level,
			OrigHash:   dep.Hash,
			Deps:       pkgs.Names(dep.DirectDeps),
			AlsoUpdate: pkgs.Names(dep.AlsoUpdate),
			Indirect:   pkgs.Names(dep.IndirectDeps),
		})
	}
	sort.Slice(todoList, func(i, j int) bool { return todoList[i].Less(todoList[j]) })
	//encoder := json.NewEncoder(os.Stdout)
	//encoder.Encode(todoList)
	level := 0
	for _, todo := range todoList {
		if level != todo.Level {
			fmt.Printf("\n")
			level++
		}
		fmt.Printf("%s :: %s\n", todo.Name, strings.Join(todo.Deps, " "))
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
