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
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gopkg.in/fsnotify.v1"
)

var (
	watchedRun = flag.String("r", "", "main go file to run")
	watchedFmt = "."
	noRun      = flag.Bool("n", false, "only run gofmt on watched Go files")
	inFile     = flag.String("i", "", "input file")

	delay      = flag.Int("d", 1, "delay time before detecting file change")
	isPipe     = false
	exitCode   = 0
	lastReport = make(map[string]time.Time)
)

func getPackageNameAndImport(sourceName string) (packageName string, imports []string) {
	fset := token.NewFileSet() // positions are relative to fset

	// parse the file containing this very example
	// but stop after processing the imports.
	f, err := parser.ParseFile(fset, sourceName, nil, parser.ImportsOnly)
	if err != nil {
		fmt.Println(err)
		return
	}

	// print the imports from the file's AST.
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

func format(path string) error {
	log.Println("fmt", path)
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

var goRunCmd *exec.Cmd

func runGoRun(sourceName string) {
	killGoRun()
	goRunCmd = exec.Command("go", "run", sourceName)
	func(goRunCmd *exec.Cmd) {
		if len(*inFile) > 0 {
			f, err := os.Open(*inFile)
			if err != nil {
				goRunCmd.Stdin = os.Stdin
			} else {
				goRunCmd.Stdin = f
				defer f.Close()
			}
		}
		goRunCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		goRunCmd.Stdout = os.Stdout
		goRunCmd.Stderr = os.Stderr
		goRunCmd.Start()
		log.Println("run", sourceName)
		goRunCmd.Wait()
	}(goRunCmd)
}

func killGoRun() {
	if goRunCmd != nil && goRunCmd.Process != nil && goRunCmd.ProcessState == nil {
		// to properly kill go run and its children
		// we need to set goRunCmd.SysProcAttr.Setid to true
		// and send kill signal to the process group
		// with negative PID
		syscall.Kill(-goRunCmd.Process.Pid, syscall.SIGKILL)
		goRunCmd.Process.Wait()
	}
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

	if len(*watchedRun) == 0 {
		if len(mainFiles) == 1 {
			*watchedRun = mainFiles[0]
			log.Println("found a main Go file, watch and run", mainFiles[0])
		} else if len(mainFiles) > 1 {
			*watchedRun = mainFiles[0]
			log.Println("found more than one main Go files, watch and run", mainFiles[0])
		} else {
			*watchedRun = ""
			log.Println("main Go files not found")
		}
	} else {
		*watchedRun, _ = filepath.Abs(*watchedRun)
	}

	if len(*watchedRun) > 0 && !*noRun {
		go runGoRun(*watchedRun)
		defer killGoRun()
	}

	log.Println("watching", path)

	fi, _ := os.Stdout.Stat()
	isPipe = fi.Mode()&os.ModeNamedPipe != 0

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill)

	for {
		select {
		case <-sig:
			return
		case event, more := <-watcher.Events:
			if more {
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
							if event.Name == *watchedRun && !*noRun {
								go runGoRun(*watchedRun)
							}
						}
					}
				}
			} else {
				return
			}
		}
	}
}
