package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gopkg.in/fsnotify.v1"
)

var (
	watchPath  = "."
	noRun      = flag.Bool("n", false, "only run gofmt")
	delay      = flag.Int("d", 1, "delay time before detecting file change")
	isPipe     = false
	exitCode   = 0
	lastReport = make(map[string]time.Time)
)

// func isMainPackage(f os.File) Bool {

// }

// func hasMainFunction(f os.File) Bool {

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

// func isGoFile(f os.FileInfo) bool {
// 	// ignore non-Go files
// 	name := f.Name()
// 	return !f.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
// }

func log(a ...interface{}) {
	var b []interface{}
	b = append(b, "\033[33mgowatch:")
	b = append(b, a...)
	b = append(b, "\033[0m")
	fmt.Fprintln(os.Stderr, b...)
}

func main() {
	gowatchMain()
	os.Exit(exitCode)
}

func gowatchMain() {
	flag.Parse()

	if flag.NArg() > 0 {
		watchPath = flag.Arg(0)
	}

	path, err := filepath.Abs(watchPath)
	if err != nil {
		panic(err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	err = watcher.Add(path)
	if err != nil {
		panic(err)
	}
	defer watcher.Close()

	log("watching", path)

	command := exec.Command("gofmt", "-w", path)
	command.Run()

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
				if filepath.Ext(event.Name) == ".go" {
					log("gofmt -w", relPath)
					command := exec.Command("gofmt", "-w", event.Name)
					command.Run()
					if packageName, _ := getPackageNameAndImport(event.Name); packageName == "main" && !*noRun {
						log("run", relPath)
						command = exec.Command("go", "run", event.Name)
						command.Stdout = os.Stdout
						command.Run()
					}
				}
			}
		}
	}
}
