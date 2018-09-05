package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	//"strings"
)

type Todo struct {
	Name       string
	Path       string
	Level      int
	OrigHash   Hash     `json:",omitempty"`
	Deps       []string `json:",omitempty"`
	AlsoUpdate []string `json:",omitempty"`
	Indirect   []string `json:",omitempty"`

	NewHash    Hash            `json:",omitempty"`
	NewVersion string          `json:",omitempty"`
	NewDeps    map[string]Hash `json:",omitempty"`

	published bool // published and in a valid state
	next      bool // all name deps published
}

func (x *Todo) ClearState() {
	x.NewHash = ""
	x.NewVersion = ""
	x.NewDeps = nil
	x.published = false
	x.next = false
}

type TodoList []*Todo
type TodoByName map[string]*Todo

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

func Gather(pkgName string) (pkgs Packages, todoList TodoList, err error) {
	pkgs = Packages{}
	_, err = GatherDeps(pkgs, "", ".")
	if err != nil {
		err = fmt.Errorf("could not gather deps: %s", err.Error())
		return
	}
	//pkgs.Dump()
	target := pkgs.ByName(pkgName)
	if target == nil {
		err = fmt.Errorf("package not found: %s", pkgName)
		return
	}
	lst := BubbleList(pkgs, target.Hash)
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
	return
}

func ReadTodo() (lst TodoList, err error) {
	fn := os.Getenv("GX_UPDATE_STATE")
	if fn == "" {
		err = fmt.Errorf("GX_UPDATE_STATE not set")
		return
	}
	bytes, err := ioutil.ReadFile(fn)
	if err != nil {
		return
	}
	err = json.Unmarshal(bytes, &lst)
	return
}

func (todoList TodoList) ToJSON(out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(todoList)
}

// Write writes the contents back to disk, file must already exist as
// a safety mechanism
func (todoList TodoList) Write() error {
	fn := os.Getenv("GX_UPDATE_STATE")
	if fn == "" {
		return fmt.Errorf("GX_UPDATE_STATE not set")
	}
	f, err := os.OpenFile(fn, os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	return todoList.ToJSON(f)
}

func (todoList TodoList) CreateMap() (TodoByName, error) {
	byName := TodoByName{}
	for _, v := range todoList {
		prev, ok := byName[v.Name]
		if ok {
			// FIXME: Maybe be a little more permissive...
			return nil, fmt.Errorf("duplicate entries for %s: %s and %s", v.Name, prev.OrigHash, prev.OrigHash)
		}
		byName[v.Name] = v
	}
	return byName, nil
}

func GetTodo() (lst TodoList, byName TodoByName, err error) {
	lst, err = ReadTodo()
	if err != nil {
		return
	}
	byName, err = lst.CreateMap()
	if err != nil {
		return
	}
	UpdateState(lst, byName)
	return
}

func UpdateState(lst TodoList, byName TodoByName) {
	for _, todo := range lst {
		if todo.NewHash != "" {
			todo.published = true
		}
		for name, hash := range todo.NewDeps {
			if !byName[name].published || byName[name].NewHash != hash {
				todo.published = false
			}
		}
		todo.next = true
		for _, name := range todo.Deps {
			if !byName[name].published {
				todo.next = false
			}
		}
	}
}
