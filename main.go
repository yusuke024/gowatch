package main

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"gopkg.in/fsnotify.v1"
)

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

	for event := range watcher.Events {
		fmt.Println(event)
		if event.Op == fsnotify.Create || event.Op == fsnotify.Write {
			if filepath.Ext(event.Name) == ".go" {
				fmt.Println("Formatting")
				command := exec.Command("gofmt", "-w", event.Name)
				command.Start()
				command.Wait()
			}
		}
	}
}
