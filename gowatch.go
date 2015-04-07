package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/fsnotify.v1"
)

type ColorLogger log.Logger

const (
	logPrefix  = "\033[33m"
	logPostfix = "\033[0m"
)

var (
	watchedRun = ""
	watchedFmt = "."
	noRun      = flag.Bool("n", false, "only run gofmt")
	delay      = flag.Int("d", 1, "delay time before detecting file change")
	isPipe     = false
	exitCode   = 0
	lastReport = make(map[string]time.Time)
)

// func hasMainFunction(f os.File) Bool {
//
// }

func getPackageNameAndImport(sourceName string) (packageName string, imports []string) {
	fset := token.NewFileSet() // positions are relative to fset

	// Parse the file containing this very example
	// but stop after processing the imports.
	f, err := parser.ParseFile(fset, sourceName, nil, parser.ImportsOnly)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Print the imports from the file's AST.
	packageName = f.Name.Name

	for _, s := range f.Imports {
		importedPackageName := s.Path.Value[1 : len(s.Path.Value)-1]
		imports = append(imports, importedPackageName)
	}

	return
}

func getMainFile(path string) string {
	fi, _ := ioutil.ReadDir(path)
	for _, f := range fi {
		fmt.Println(f.Name())
		if filepath.Ext(f.Name()) == ".go" {
			fmt.Println(isMainPackage(f.Name()))
		}
	}
	return ""
}

func isGoFile(f os.FileInfo) bool {
	// ignore non-Go files
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
}

func isMainPackage(sourceName string) bool {
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, sourceName, nil, parser.ImportsOnly)

	return f.Name.Name == "main"
}

func isMainFile(sourceName string) bool {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, sourceName, nil, 0)
	if err != nil {
		return false
	}

	if o := f.Scope.Lookup("main"); f.Name.Name == "main" && o != nil && o.Kind == ast.Fun {
		return true
	}

	return false
}

func visitFile(path string, info os.FileInfo, err error) error {
	if isGoFile(info) {
		format(path)
		if isMainFile(path) {
			fmt.Println("main file -", path)
		}
	}
	return nil
}

func format(path string) error {
	log.Println("gofmt -w", path, logPostfix)
	command := exec.Command("gofmt", "-w", path)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Run()
	return nil
}

func main() {
	gowatchMain()
	os.Exit(exitCode)
}

func gowatchMain() {
	flag.Parse()

	log.SetPrefix(logPrefix)

	path, err := filepath.Abs(watchedFmt)
	if err != nil {
		panic(err)
	}

	// switch flag.NArg() {
	// case 0:
	// case 1:
	// 	watchedRun = flag.Arg(0)
	// case 2:
	// default:
	// }
	//
	// watchedRun = getMainFile(path)
	// fmt.Println(watchedRun)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	err = watcher.Add(path)
	if err != nil {
		panic(err)
	}
	defer watcher.Close()

	log.Println("watching", path, logPostfix)

	filepath.Walk(path, visitFile)

	fi, _ := os.Stdout.Stat()
	isPipe = fi.Mode()&os.ModeNamedPipe != 0

	for event := range watcher.Events {
		if event.Op == fsnotify.Create || event.Op == fsnotify.Write {
			if time.Since(lastReport[event.Name]) < time.Duration(*delay)*time.Second {
				continue
			}
			lastReport[event.Name] = time.Now()
			relPath, _ := filepath.Rel(path, event.Name)
			if isPipe {
				io.WriteString(os.Stdout, relPath+"\n")
			} else {
				if f, _ := os.Stat(event.Name); isGoFile(f) {
					log.Println("gofmt -w", relPath, logPostfix)
					format(event.Name)
					// if packageName, _ := getPackageNameAndImport(event.Name); packageName == "main" && !*noRun {
					// 	log.Println("run", relPath, logPostfix)
					// 	command = exec.Command("go", "run", event.Name)
					// 	command.Stdout = os.Stdout
					// 	command.Stderr = os.Stderr
					// 	command.Run()
					// }
				}
			}
		}
	}
}
