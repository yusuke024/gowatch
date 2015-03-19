package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/fsnotify.v1"
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

func main() {
	path, err := filepath.Abs(".")
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

	fmt.Println("Watching:", path)

	command := exec.Command("gofmt", "-w", ".")
	command.Run()

	for event := range watcher.Events {
		if event.Op == fsnotify.Create || event.Op == fsnotify.Write {
			if filepath.Ext(event.Name) == ".go" {
				command := exec.Command("gofmt", "-w", event.Name)
				command.Run()

				packageName, _ := getPackageNameAndImport(event.Name)
				if packageName == "main" {
					fmt.Println("Run file:", event.Name)
					command = exec.Command("go", "run", event.Name)
					command.Stdout = os.Stdout
					command.Run()
				}
			}
		}
	}
}
