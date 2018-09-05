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
	if len(os.Args) <= 1 {
		return fmt.Errorf("usage: %s rev-deps|init|next|published|update-list|to-pin ...", os.Args[0])
	}
	switch os.Args[1] {
	case "rev-deps":
		usage := func() error {
			return fmt.Errorf("usage: %s rev-deps [--json|--list] <name>", os.Args[0])
		}
		if len(os.Args) <= 2 {
			return usage()
		}
		mode := ""
		name := ""
		for _, arg := range os.Args[2:] {
			if len(arg) > 2 && arg[0:2] == "--" {
				switch arg[2:] {
				case "json", "list":
					mode = arg[2:]
				default:
					return usage()
				}
			} else if name != "" {
				return usage()
			} else {
				name = arg
			}
		}
		if name == "" {
			return usage()
		}
		var todoList TodoList
		if os.Getenv("GX_UPDATE_STATE") == "" {
			_, todoList, err = Gather(name)
			if err != nil {
				return err
			}
		} else {
			lst, err := ReadTodo()
			if err != nil {
				return err
			}
			var todo *Todo
			for _, todo = range lst {
				if todo.Name == name {
					break
				}
			}
			if todo == nil {
				return fmt.Errorf("package not found: %s", name)
			}
			deps := NameSet{}
			deps.Add(name)
			deps.Add(todo.Deps...)
			deps.Add(todo.AlsoUpdate...)
			deps.Add(todo.Indirect...)
			for _, todo = range lst {
				if !deps.Has(todo.Name) {
					continue
				}
				todo.ClearState()
				todoList = append(todoList, todo)
			}
		}
		switch mode {
		case "":
			level := 0
			for _, todo := range todoList {
				if level != todo.Level {
					fmt.Printf("\n")
					level++
				}
				fmt.Printf("%s :: %s\n", todo.Path, strings.Join(todo.Deps, " "))
			}
		case "json":
			return todoList.ToJSON(os.Stdout)
		case "list":
			for _, todo := range todoList {
				fmt.Printf("%s\n", todo.Path)
			}
		default:
			panic("internal error")
		}
		return nil
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
		path := filepath.Join(rootPath, ".gx-update-state.json")
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
		if err != nil {
			return err
		}
		defer f.Close()
		err = todoList.ToJSON(f)
		if err != nil {
			return err
		}
		fmt.Printf("export GX_UPDATE_STATE=%s\n", path)
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
		todo.NewVersion = lastPubVer.Version
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
		for _, hash := range hashes {
			fmt.Printf("%s\n", hash)
		}
	case "to-pin":
		todoList, _, err := GetTodo()
		if err != nil {
			return err
		}
		unpublished := []string{}
		for i, todo := range todoList {
			if todo.published {
				fmt.Printf("%s %s %s\n", todo.NewHash, todo.Path, todo.NewVersion)
			} else if i != len(todoList)-1 {
				// ^^ ignore very last item in the list as it the final
				// target and does not necessary need to be gx
				// published
				unpublished = append(unpublished, todo.Name)
			}
		}
		if len(unpublished) > 0 {
			return fmt.Errorf("unpublished dependencies: %s", strings.Join(unpublished, " "))
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
