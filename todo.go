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

type JsonState struct {
	Todo     []*Todo
	Defaults map[string]string `json:",omitempty"`
}

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

	Meta     map[string]string `json:",omitempty"`
	defaults map[string]string // shared among all todo entries

	published bool // published and in a valid state
	ready     bool // all name deps published
}

func (x *Todo) ClearState() {
	x.NewHash = ""
	x.NewVersion = ""
	x.NewDeps = nil
	x.published = false
	x.ready = false
	x.Meta = nil
	x.defaults = nil
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

type NotYetPublished struct {
	Todo *Todo
	Key  string
}

func (e NotYetPublished) Error() string {
	return fmt.Sprintf("%s: '%s' undefined, not yet published", e.Todo.Path, e.Key)
}

func (v *Todo) Get(key string) (val string, have bool, err error) {
	switch key {
	case "name":
		val = v.Name
		have = true
	case "path":
		val = v.Path
		have = true
	case "ver", "version":
		if !v.published {
			err = NotYetPublished{v, key}
			return
		}
		val = v.NewVersion
		have = true
	case "hash":
		if !v.published {
			err = NotYetPublished{v, key}
			return
		}
		val = string(v.NewHash)
		have = true
	case "published":
		if v.published {
			val = "="+string(v.NewHash)
			have = true
		}
		// default empty string, no error
	case "ready":
		if v.ready {
			val = "READY"
			have = true
		}
		// default empty string, no error
	default:
		val, have = v.Meta[key]
		if have {
			return
		}
		var ok bool
		val, ok = v.defaults[key]
		if ok {
			return
		}
		err = fmt.Errorf("%s: '%s' undefined", v.Path, key)
	}
	return
}

func CheckInternal(key string) error {
	switch key {
	case "name", "path", "level", "ver", "version", "hash", "published", "ready", "deps":
		return fmt.Errorf("cannot set internal value: %s", key)
	}
	return nil
}

func (v *Todo) Set(key string, val string) error {
	if err := CheckInternal(key); err != nil {
		return err
	}
	v.Meta[key] = val
	return nil
}

func (v *Todo) Unset(key string) error {
	if err := CheckInternal(key); err != nil {
		return err
	}
	delete(v.Meta, key)
	return nil
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

func ReadStateFile() (state JsonState, err error) {
	fn := os.Getenv("GX_UPDATE_STATE")
	if fn == "" {
		err = fmt.Errorf("GX_UPDATE_STATE not set")
		return
	}
	bytes, err := ioutil.ReadFile(fn)
	if err != nil {
		return
	}
	err = json.Unmarshal(bytes, &state)
	return
}

func Encode(out io.Writer, v interface{}) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
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
	state := JsonState{Todo: todoList, Defaults: todoList[0].defaults}
	return Encode(f, state)
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
	state, err := ReadStateFile()
	if err != nil {
		return
	}
	lst = state.Todo
	defaults := state.Defaults
	if defaults == nil {
		defaults = map[string]string{}
	}
	for _, todo := range lst {
		todo.defaults = defaults
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
		if todo.published {
			todo.ready = false
			continue
		}
		todo.ready = true
		for _, name := range todo.Deps {
			if !byName[name].published {
				todo.ready = false
			}
		}
	}
}
