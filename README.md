# gowatch

Go files watcher for Gophers and everyone else.

A tiny Go program that can:
- Run `gofmt` on saving Go files
- Run a "main" file and restart when it changes.

## Install

If you already have your *Go* workspace and `GOPATH` set, simply run:

```
go get -u github.com/sikhapol/gowatch
```

## How to use

### Simple

- Run `gofmt -w` upon saving *Go* files.
- Run `main` function when the file is saved.

```
gowatch
```

### Specify watched directory

```
gowatch /path/to/watch/
```

### Specify Go file to run
```
gowatch -r /path/to/main.go
```

### Specify `stdin` for the run main file
```
gowatch -i input.txt
```

### Format only

```
gowatch -n
```

### Delay reporting file change

```
gowatch -d 5 #seconds
```
This mean that if a file has changed, gowatch will not take any action if it's changed again within 5 seconds. This is to prevent a loop that cause by gofmt changing the file that cause gowatch to detect change and format the file again and so on. Default is 1 second.

### For everything else

`gowatch` knows if you send input to another command with UNIX pipe.
In this case, it will just print the changed file names to `stdout` which you can then send to another command like `xargs` to further do anything interesting.

```
gowatch | xargs cat
```

## License
MIT
