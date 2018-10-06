package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
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

var args = os.Args[1:]
var curCmd *Command

func Shift() (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	arg := args[0]
	args = args[1:]
	return arg, true
}

type Command struct {
	Name    string
	Tagline string
	Usage   string
	Help    string
	Run     func() error
}

var reqGxUpdateState = "\nRequires the GX_UPDATE_STATE env. variable to be set.  See init sub-command."

func UsageErr() error {
	return fmt.Errorf("%s %s\n", os.Args[0], curCmd.Usage)
}

var cmds = []*Command{
	&previewCmd,
	&initCmd,
	&statusCmd,
	&stateCmd,
	&listCmd,
	&depsCmd,
	&publishedCmd,
	&toPinCmd,
	&metaCmd,
}

func mainFun() error {
	err := InitGlobal()
	if err != nil {
		return err
	}
	cmd := ""
	rest := []string{}
	showHelp := false
	for len(args) != 0 {
		arg, _ := Shift()
		switch {
		case arg == "-h" || arg == "--help":
			showHelp = true
		case len(arg) > 0 && arg[0] == '-':
			rest = append(rest, arg)
		case cmd == "":
			cmd = arg
		default:
			rest = append(rest, arg)
		}
	}
	args = rest
	usageErr := fmt.Errorf("Usage: %s [-h] preview|init|status|list|deps|published|to-pin|meta", os.Args[0])
	if !showHelp && cmd == "" {
		return usageErr
	}
	if showHelp && cmd == "" {
		fmt.Printf("%s\n\n", usageErr.Error())
		for _, c := range cmds {
			fmt.Printf("  %-10s %s\n", c.Name, c.Tagline)
		}
		fmt.Printf("\n")
		return nil
	}
	for _, c := range cmds {
		if c.Name == cmd {
			curCmd = c
			break
		}
	}
	if curCmd == nil {
		return fmt.Errorf("unknown command: %s", cmd)
	}
	if showHelp {
		fmt.Printf("Usage: %s %s\n", os.Args[0], curCmd.Usage)
		fmt.Printf("%s\n", curCmd.Help)
		return nil
	} else {
		return curCmd.Run()
	}
}

var previewCmd = Command{
	Name:    "preview",
	Tagline: "Show dep. that need to be changed to change <dep> in current package",
	Usage:   "preview [--json|--list] [-f <fmtstr>] <dep>",
	Help: `
Show decencies that need to be changed in order to change <dep> in the
current package.  The normal output lists each decency and what that
decency directly depends on.  If --json is given a more detailed
output is given is using JSON.  If --list is given just the decencies
are listed.

The -f option can be used to customize the output.  It defaults to
'$path[ :: $deps]' for the normal output and '$path' if the --list
option is given.
` + FormatHelp(BasicKeys),
	Run: previewCmdRun,
}

func previewCmdRun() error {
	var err error
	if len(os.Args) <= 2 {
		return UsageErr()
	}
	mode := ""
	name := ""
	fmtstr := ""
	for len(args) > 0 {
		arg, _ := Shift()
		switch arg {
		case "--json":
			mode = "json"
		case "--list":
			mode = "list"
		case "-f":
			arg, ok := Shift()
			if !ok {
				UsageErr()
			}
			fmtstr = arg
		default:
			if arg == "" || arg[0] == '-' {
				return UsageErr()
			}
			name = arg
		}
	}
	_, todoList, _, err := Gather(name)
	if err != nil {
		return err
	}
	switch mode {
	case "":
		if fmtstr == "" {
			fmtstr = "$path[ :: $deps]"
		}
		level := 0
		for _, todo := range todoList {
			if level != todo.Level {
				fmt.Printf("\n")
				level++
			}
			str, err := todo.Format(fmtstr)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", str)
		}
	case "json":
		return Encode(os.Stdout, todoList)
	case "list":
		if fmtstr == "" {
			fmtstr = "$path"
		}
		for _, todo := range todoList {
			str, err := todo.Format(fmtstr)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", str)
		}
	default:
		panic("internal error")
	}
	return nil
}

var initCmd = Command{
	Name:    "init",
	Tagline: "Starts a new session for updating <dep> in the current package",
	Usage:   "init <name>",
	Help: `
Starts a new session for updating <dep> in the current package.  It
creates a JSON file. '.gx-update-state.json', to keep track of the
current state in the curent directory.  All command except this one
and 'preview' expect the location to be set in the GX_UPDATE_STATE
environmental variable.  

The command will output the necessary command to set this variable to
the correct value for Bourne shells
`,
	Run: initCmdRun,
}

func initCmdRun() error {
	if len(os.Args) != 3 {
		return fmt.Errorf("usage: %s init <dep>", os.Args[0])
	}
	pkgs, todoList, orig, err := Gather(os.Args[2])
	if err != nil {
		return err
	}
	byName, err := todoList.CreateMap()
	if err != nil {
		return err
	}
	UpdateState(todoList, byName, orig)
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
	state := JsonState{Todo: todoList, Orig: orig}
	err = Encode(f, state)
	if err != nil {
		return err
	}
	fmt.Printf("export GX_UPDATE_STATE=%s\n", path)
	return nil
}

var statusCmd = Command{
	Name:    "status",
	Tagline: "Show current status.",
	Usage:   "status",
	Help: `
Show current status.

Alias for: list -f '$path[ ($invalidated)][ = $hash][ $ready][ :: $unmet][ DUP DEPS FOR: $dups]' --by-level
` + reqGxUpdateState,
	Run: func() error {
		args = []string{"-f", "$path[ ($invalidated)][ = $hash][ $ready][ :: $unmet][ DUP DEPS FOR: $dups]", "--by-level"}
		return listCmdRun()
	},
}

var stateCmd = Command{
	Name:    "state",
	Tagline: "Show state as JSON file",
	Usage:   "state",
	Help: `
Show state as JSON file
` + reqGxUpdateState,
	Run: func() error {
		fn := os.Getenv("GX_UPDATE_STATE")
		if fn == "" {
			return fmt.Errorf("GX_UPDATE_STATE not set")
		}
		bytes, err := ioutil.ReadFile(fn)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(bytes)
		return err
	},
}

var listCmd = Command{
	Name:    "list",
	Tagline: "Lists all dep. optionally matching a condition in useful ways",
	Usage:   "list [-f <fmtstr>] [--by-level] [not] [ready|published|<user-cond>]",
	Help: `
Lists all the dep. optionally matching a condition in useful ways.

The -f option can be used to custom the output and defaults to '$path'.

The --by-level option groups the dep. based on level in the
reverse dep. graph.
` + FormatHelp(AllKeys) + `
EXAMPLES

To list all packages that are ready to be updated by directory:
  gx-update-helper deps ready -f '$dir'
` + reqGxUpdateState,
	Run: listCmdRun,
}

func listCmdRun() error {
	var ok bool
	invert := false
	cond := ""
	fmtstr := "$path"
	bylevel := false
	for len(args) > 0 {
		arg, _ := Shift()
		switch arg {
		case "-f":
			fmtstr, ok = Shift()
			if !ok {
				return UsageErr()
			}
		case "not":
			invert = true
			cond, ok = Shift()
			if !ok {
				return UsageErr()
			}
		case "--by-level":
			bylevel = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return UsageErr()
			}
			cond = arg
			break
		}
	}
	if len(args) != 0 {
		return UsageErr()
	}
	lst, _, _, err := GetTodo()
	if err != nil {
		return err
	}
	errors := false
	level := -1
	for _, todo := range lst {
		ok := true
		if cond != "" {
			_, ok, _ = todo.Get(cond)
		}
		if invert {
			ok = !ok
		}
		if ok {
			str, err := todo.Format(fmtstr)
			if err == BadFormatStr {
				return err
			} else if err != nil {
				fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
				errors = true
				continue
			}
			if bylevel && level != -1 && todo.Level != level {
				os.Stderr.Write([]byte("\n"))
			}
			level = todo.Level
			fmt.Printf("%s\n", str)
		}
	}
	if errors {
		return fmt.Errorf("some entries could not be displayed")
	}
	return nil
}

var depsCmd = Command{
	Name:    "deps",
	Tagline: "List dep. of current package",
	Usage:   "deps [-f <fmtstr>] [-p <pkg>] [direct] [also] [to-update] [indirect] [all]",
	Help: `
List dependencies of current or specified package.  The '-p' option
specifies the package to use.  If it is omitted the current package is
used instead.

If no additional arguments are given list the direct dep.  Otherwise
lists the depences as given by the following arguments:
  direct:
  also: non-direct dep. also listed in package.json
  to-update|specified: all dep. listed on package.json
  indirect:
  all:

If the -f option is omitted, it defaults to '$path'.
` + FormatHelp(AllKeys) + reqGxUpdateState,
	Run: depsCmdRun,
}

func depsCmdRun() error {
	fmtstr := "$path"
	pkgName := ""
	which := map[int]string{}
	for len(args) > 0 {
		arg, _ := Shift()
		switch arg {
		case "direct":
			which[1] = "direct"
		case "also":
			which[2] = "also"
		case "to-update", "specified":
			which[1] = "direct"
			which[2] = "also"
		case "indirect":
			which[3] = "indirect"
		case "all":
			which[1] = "direct"
			which[2] = "also"
			which[3] = "indirect"
		case "-f":
			arg, ok := Shift()
			if !ok {
				return UsageErr()
			}
			fmtstr = arg
		case "-p":
			arg, ok := Shift()
			if !ok {
				return UsageErr()
			}
			pkgName = arg
		default:
			return UsageErr()
		}
	}
	if len(which) == 0 {
		which[1] = "direct"
	}

	_, byName, _, err := GetTodo()
	if err != nil {
		return err
	}
	if pkgName == "" {
		pkg, err := ReadPackage(".")
		if err != nil {
			return err
		}
		pkgName = pkg.Name
	}
	todo, ok := byName[pkgName]
	if !ok {
		return fmt.Errorf("could not find entry for %s", pkgName)
	}

	deps := []string{}
	for _, d := range which {
		switch d {
		case "direct":
			deps = append(deps, todo.Deps...)
		case "also":
			deps = append(deps, todo.AlsoUpdate...)
		case "indirect":
			deps = append(deps, todo.Indirect...)
		default:
			panic("internal error")
		}
	}
	sort.Strings(deps)
	errors := false
	var buf bytes.Buffer
	for _, dep := range deps {
		todo := byName[dep]
		str, err := todo.Format(fmtstr)
		if err == BadFormatStr {
			return err
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
			errors = true
			continue
		}
		buf.Write(str)
		buf.WriteByte('\n')
	}
	if errors {
		return fmt.Errorf("aborting due to previous errors")
	}
	os.Stdout.Write(buf.Bytes())
	return nil
}

var publishedCmd = Command{
	Name:    "published",
	Tagline: "change the published state of a package",
	Usage:   "published reset|clean",
	Help: `
Change the publihsed state of a package.

With no arguments the current package will be mark as potently being
published with the hash as given in .gx/lastpubver.  It also record
the hash of the deps as given in package.json.  The package will only
be marked as published if all those hashes match the recorded
published hash, otherwise the package will be marked as being in an
invalidated state.

If the 'reset' argument is given then clear the published into.

If the 'clean' option is given remove the published info state of ALL
packages in an invalidated state.
` + reqGxUpdateState,
	Run: publishedCmdRun,
}

func publishedCmdRun() error {
	mode := "mark"
	if len(args) > 0 {
		mode, _ = Shift()
	}
	if len(args) > 0 {
		return UsageErr()
	}
	todoList, todoByName, orig, err := GetTodo()
	if err != nil {
		return err
	}
	var finalErr error
	switch mode {
	case "clean":
		for _, todo := range todoList {
			if todo.Published {
				continue
			}
			todo.NewHash = ""
			todo.NewVersion = ""
			todo.NewDeps = nil
		}
		UpdateState(todoList, todoByName, orig)
	case "mark", "reset":
		pkg, lastPubVer, err := GetGxInfo()
		if err != nil {
			return err
		}
		todo, ok := todoByName[pkg.Name]
		if !ok {
			return fmt.Errorf("could not find entry for %s", pkg.Name)
		}
		switch mode {
		case "mark":
			todo.NewHash = lastPubVer.Hash
			todo.NewVersion = lastPubVer.Version
			depMap := map[string]Hash{}
			for _, dep := range pkg.GxDependencies {
				if hash, ok := depMap[dep.Name]; ok {
					return fmt.Errorf("duplicate dependency %s: %s %s", dep.Name, hash, dep.Hash)
				}
				depMap[dep.Name] = dep.Hash
			}
			todo.NewDeps = depMap
			UpdateState(todoList, todoByName, orig)
			if mode == "mark" && !todo.Published {
				finalErr = fmt.Errorf("could not put %s in published state, run '%s status' for more info",
					todo.Name, os.Args[0])
			}
		case "reset":
			todo.NewHash = ""
			todo.NewVersion = ""
			todo.NewDeps = nil
			UpdateState(todoList, todoByName, orig)
		}
	default:
		return UsageErr()
	}
	err = todoList.Write()
	if err != nil {
		return err
	}
	return finalErr
}

var toPinCmd = Command{
	Name:    "to-pin",
	Tagline: "list the pins of packages once done",
	Usage:   "to-pin -f <fmtstr>",
	Help: `
List the pins of all packages once done.  It will return an error if
all but the last package is not yet publicized.

The default value for -f is '$hash $path $version'
` + FormatHelp(AllKeys) + reqGxUpdateState,
	Run: toPinCmdRun,
}

func toPinCmdRun() error {
	var ok bool
	fmtstr := "$hash $path $version"
	for len(args) > 0 {
		arg, _ := Shift()
		switch arg {
		case "-f":
			fmtstr, ok = Shift()
			if !ok {
				return UsageErr()
			}
		default:
			return UsageErr()
		}
	}
	todoList, _, _, err := GetTodo()
	if err != nil {
		return err
	}
	unpublished := []string{}
	for i, todo := range todoList {
		if todo.Published {
			str, err := todo.Format(fmtstr)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", str)
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
	return nil
}

var metaCmd = Command{
	Name:    "meta",
	Tagline: "Change the state of meta-data for a package.",
	Usage:   "meta [-p <pkg>] get|set|unset|vals|default ...",
	Help: `
Manipulate the state of meta-data for a package.

The current package is used unless the '-p' option is given.  The
following subcommands are provided:

  get <key>
  unset <key> <val>
  set <key>
  vals: list all key/val pairs
  default get|unset|set|vals: change the default state
` + reqGxUpdateState,
	Run: metaCmdRun,
}

func metaCmdRun() error {
	lst, byName, _, err := GetTodo()
	if err != nil {
		return err
	}
	arg, ok := Shift()
	if !ok {
		return UsageErr()
	}
	pkgName := ""
	notUsed := make([]string, 0, len(args))
	for len(args) > 0 {
		arg, _ := Shift()
		if arg == "-p" {
			arg, ok := Shift()
			if !ok {
				return UsageErr()
			}
			pkgName = arg
		} else {
			notUsed = append(notUsed, arg)
		}
	}
	args = notUsed
	modified := false
	if arg == "default" {
		arg, ok := Shift()
		if !ok {
			return fmt.Errorf("usage: %s meta default get|set|unset|vals ...", os.Args[0])
		}
		modified, err = getSetEtc(arg, lst[0].defaults, nil, "meta default")
		if err != nil {
			return err
		}
	} else {
		if pkgName == "" {
			pkg, err := ReadPackage(".")
			if err != nil {
				return err
			}
			pkgName = pkg.Name
		}
		todo, ok := byName[pkgName]
		if !ok {
			return fmt.Errorf("could not find entry for %s", pkgName)
		}
		if todo.Meta == nil {
			todo.Meta = map[string]string{}
		}
		modified, err = getSetEtc(arg, todo.Meta, todo.defaults, "meta")
		if err != nil {
			return err
		}
	}
	if modified {
		err = lst.Write()
		if err != nil {
			return err
		}
	}
	return nil
}

func getSetEtc(arg string, vals map[string]string, defaults map[string]string, prefix string) (modified bool, err error) {
	switch arg {
	case "get":
		key, ok := Shift()
		if !ok || len(args) != 0 {
			err = fmt.Errorf("usage: %s %s get <key>", os.Args[0], prefix)
			return
		}
		val, ok := vals[key]
		if !ok && defaults != nil {
			val, ok = defaults[key]
		}
		if !ok {
			err = fmt.Errorf("%s not defined", key)
			return
		}
		fmt.Printf("%s\n", val)
	case "set":
		key, _ := Shift()
		val, ok := Shift()
		if !ok || len(args) != 0 {
			err = fmt.Errorf("usage: %s %s set <key> <val>", os.Args[0], prefix)
			return
		}
		err = CheckInternal(key)
		if err != nil {
			return
		}
		vals[key] = val
		modified = true
	case "unset":
		key, ok := Shift()
		if !ok || len(args) != 0 {
			err = fmt.Errorf("usage: %s %s unset <key>", os.Args[0], prefix)
			return
		}
		err = CheckInternal(key)
		if err != nil {
			return
		}
		delete(vals, key)
		modified = true
	case "vals":
		for k, v := range vals {
			fmt.Printf("%s %s\n", k, v)
		}
	default:
		err = fmt.Errorf("expected one of: get set unset vals, got: %s", arg)
		return
	}
	return
}

func main() {
	err := mainFun()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
