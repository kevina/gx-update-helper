package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
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

	UnmetDeps []string `json:",omitempty"`

	NewHash    Hash            `json:",omitempty"`
	NewVersion string          `json:",omitempty"`
	NewDeps    map[string]Hash `json:",omitempty"`

	Meta     map[string]string `json:",omitempty"`
	defaults map[string]string // shared among all todo entries

	Published bool // published and in a valid state
	Ready     bool // all name deps published

	others TodoByName // shared among all todo entries
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

type KeyDesc struct {
	Name   string
	Alias  string
	Desc   string
	Unused bool
}

var BasicKeys = []KeyDesc{
	{Name: "name", Desc: "package name"},
	{Name: "path", Desc: "import path"},
	{Name: "dir", Desc: "directory package is located in"},
	{Name: "giturl", Desc: "git url for downloading packages"},
	{Name: "deps", Desc: "space sperated list of direct deps."},
}

var AllKeys = append(BasicKeys, []KeyDesc{
	{Name: "ready", Desc: "the string READY if all deps. are published"},
	{Name: "published", Desc: "the string PUBLISHED if published"},
	{Name: "invalidated", Desc: "the string INVALIDATED if invalidated"},
	{Name: "ver", Desc: "current version if published", Alias: "version"},
	{Name: "hash", Desc: "current hash if published"},
	{Name: "unmet", Desc: "space seperated list of unmet deps.", Alias: "unmetdeps"},
	{Name: "level", Unused: true},
}...)

func KeysHelp(keys []KeyDesc) string {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	for _, kd := range keys {
		if kd.Unused {
			continue
		}
		if kd.Alias == "" {
			fmt.Fprintf(tw, "  %s\t%s\n", kd.Name, kd.Desc)
		} else {
			fmt.Fprintf(tw, "  %s|%s\t%s\n", kd.Name, kd.Alias, kd.Desc)
		}
	}
	tw.Flush()
	return buf.String()
}

func (v *Todo) Get(key string) (val string, have bool, err error) {
	switch key {
	case "name":
		val = v.Name
		have = true
	case "path":
		val = v.Path
		have = true
	case "dir":
		dirs := []string{GOPATH, "src"}
		dirs = append(dirs, strings.Split(v.Path, "/")...)
		val = filepath.Join(dirs...)
		have = true
	case "giturl":
		i := strings.IndexByte(v.Path, '/')
		if i == -1 {
			panic("ill formed path")
		}
		val = fmt.Sprintf("git@%s:%s.git", v.Path[:i], v.Path[i+1:])
		have = true
	case "ver", "version":
		if !v.Published {
			err = NotYetPublished{v, key}
			return
		}
		val = v.NewVersion
		have = true
	case "hash":
		if !v.Published {
			err = NotYetPublished{v, key}
			return
		}
		val = string(v.NewHash)
		have = true
	case "published":
		if v.Published {
			val = "PUBLISHED"
			have = true
		}
		// default empty string, no error
	case "ready":
		if v.Ready {
			val = "READY"
			have = true
		}
		// default empty string, no error
	case "deps":
		val = strings.Join(v.Deps, " ")
		have = len(v.Deps) > 0
	case "unmet", "unmetdeps":
		val = strings.Join(v.UnmetDeps, " ")
		have = len(v.UnmetDeps) > 0
	case "invalidated":
		if len(v.NewDeps) > 0 && !v.Published {
			val = "INVALIDATED"
			have = true
		}
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
	for _, kd := range AllKeys {
		if key == kd.Name || key == kd.Alias {
			return fmt.Errorf("cannot set internal value: %s", key)
		}
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
	byName, err = lst.CreateMap()
	if err != nil {
		return
	}
	for _, todo := range lst {
		todo.defaults = defaults
		todo.others = byName
	}
	return
}

func UpdateState(lst TodoList, byName TodoByName) {
	for _, todo := range lst {
		if todo.NewHash != "" {
			todo.Published = true
		}
		for name, hash := range todo.NewDeps {
			if !byName[name].Published || byName[name].NewHash != hash {
				todo.Published = false
			}
		}
		if todo.Published {
			todo.Ready = false
			continue
		}
		todo.Ready = true
		for _, name := range todo.Deps {
			if !byName[name].Published {
				todo.UnmetDeps = append(todo.UnmetDeps, name)
				todo.Ready = false
			}
		}
	}
}
