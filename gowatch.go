package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/fsnotify.v1"
)

var (
	watchedRun = ""
	watchedFmt = "."
	noRun      = flag.Bool("n", false, "only run gofmt")
	inFile     = flag.String("in", "", "input file")

	delay      = flag.Int("d", 1, "delay time before detecting file change")
	isPipe     = false
	exitCode   = 0
	lastReport = make(map[string]time.Time)
)

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
	log.Println(info.Name())
	if isGoFile(info) {
		format(path)
		if isMainFile(path) {
			fmt.Println("main file -", path)
		}
	}
	return nil
}

func format(path string) error {
	log.Println("gofmt -w", path)
	command := exec.Command("gofmt", "-w", path)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Run()
	return nil
}

func goFiles(path string) (goFiles, mainFiles []string) {
	filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if isGoFile(info) {
			goFiles = append(goFiles, path)
			if isMainFile(path) {
				mainFiles = append(mainFiles, path)
			}
		}
		return nil
	})
	return
}

var runCmd *exec.Cmd

func goRun(sourceName string) {
	if runCmd != nil && runCmd.Process != nil {
		runCmd.Process.Kill()
	}

	runCmd = exec.Command("go", "run", sourceName)
	go func(runCmd *exec.Cmd) {
		if len(*inFile) > 0 {
			f, err := os.Open(*inFile)
			if err != nil {
				runCmd.Stdin = os.Stdin
			} else {
				runCmd.Stdin = f
				defer f.Close()
			}
		} else {
			runCmd.Stdin = os.Stdin
		}
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr
		log.Println("go run", watchedRun)
		runCmd.Run()
		log.Println("exit", watchedRun)
	}(runCmd)
}

func main() {
	gowatchMain()
	os.Exit(exitCode)
}

func gowatchMain() {
	flag.Parse()

	path, err := filepath.Abs(watchedFmt)
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

	goFiles, mainFiles := goFiles(path)

	// pre run gofmt on found Go files
	for _, f := range goFiles {
		format(f)
	}

	if len(mainFiles) == 1 {
		watchedRun = mainFiles[0]
		log.Println("found a main Go file, watch and run", mainFiles[0])
	} else if len(mainFiles) > 1 {
		watchedRun = mainFiles[0]
		log.Println("found more than one main Go files, watch and run", mainFiles[0])
	} else {
		log.Println("main Go files not found")
	}

	if len(watchedRun) > 0 && !*noRun {
		goRun(watchedRun)
	}

	log.Println("watching", path)

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
					format(event.Name)
					if event.Name == watchedRun && !*noRun {
						goRun(watchedRun)
					}
				}
			}
		}
	}
}
