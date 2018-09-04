package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"io"
	//"strings"
)

type Todo struct {
	Name       string
	Path       string
	Level      int
	NewHash    Hash     `json:",omitempty"`
	NewVersion string   `json:",omitempty"`
	OrigHash   Hash     `json:",omitempty"`
	Deps       []string `json:",omitempty"`
	AlsoUpdate []string `json:",omitempty"`
	Indirect   []string `json:",omitempty"`
}

type TodoList []*Todo

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

func Gather(pkgName string) (Packages, TodoList, error) {
	pkgs := Packages{}
	_, err := GatherDeps(pkgs, "", ".")
	if err != nil {
		return nil, nil, fmt.Errorf("could not gather deps: %s", err.Error())
	}
	//pkgs.Dump()
	target := pkgs.ByName(pkgName)
	if target == nil {
		return nil, nil, fmt.Errorf("package not found: %s", os.Args[2])
	}
	lst := BubbleList(pkgs, target.Hash)
	todoList := TodoList{}
	for _, dep := range lst {
		todoList = append(todoList, &Todo{
			Name:       pkgs[dep.Hash].Name,
			Path:       pkgs[dep.Hash].Path,
			Level:      dep.Level,
			OrigHash:   dep.Hash,
			Deps:       pkgs.Names(dep.DirectDeps),
			AlsoUpdate: pkgs.Names(dep.AlsoUpdate),
			Indirect:   pkgs.Names(dep.IndirectDeps),
		})
	}
	sort.Slice(todoList, func(i, j int) bool { return todoList[i].Less(todoList[j]) })
	return pkgs, todoList, nil
}

func ReadTodo() (TodoList, error) {
	todoFile := os.Getenv("MYGX_WORKSPACE")
	if todoFile == "" {
		return nil, fmt.Errorf("GOPATH not set")
	}
	bytes, err := ioutil.ReadFile(todoFile)
	if err != nil {
		return nil, err
	}
	todo := &[]*Todo{}
	err = json.Unmarshal(bytes, todo)
	if err != nil {
		return nil, err
	}
	return *todo, nil
}

func (todoList TodoList) Write(out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(todoList)
}

func (todoList TodoList) WriteToFile (path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	defer f.Close()
	if err != nil {
		return err
	}
	return todoList.Write(f)
}
