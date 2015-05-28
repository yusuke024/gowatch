package main

import (
	"flag"
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
	watchedRun    []string
	watchedFmt, _ = os.Getwd()
	noRun         = flag.Bool("n", false, "only run gofmt on watched Go files")
	inFile        = flag.String("i", "", "input file")

	delay      = flag.Int("d", 1, "delay time before detecting file change")
	isPipe     = false
	exitCode   = 0
	lastReport = make(map[string]time.Time)
)

func isGoFile(f os.FileInfo) bool {
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
}

func isMainPackage(sourceName string) bool {
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, sourceName, nil, parser.PackageClauseOnly)

	return f.Name.Name == "main"
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
			if isMainPackage(path) {
				mainFiles = append(mainFiles, path)
			}
		}
		return nil
	})
	return
}

var (
	goRunCmd  *exec.Cmd
	firstTime = true
)

func runGoRun(sourceNames []string) {
	killGoRun()
	var args []string
	args = append(args, "run")
	args = append(args, sourceNames...)
	goRunCmd = exec.Command("go", args...)
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
		if firstTime {
			log.Println("run main package")
			firstTime = false
		} else {
			log.Println("restart main package")
		}
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

	watchedRun = append(watchedRun, mainFiles...)

	if len(watchedRun) > 0 && !*noRun {
		go runGoRun(watchedRun)
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
							if !*noRun {
								go runGoRun(watchedRun)
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
