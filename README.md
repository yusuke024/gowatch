# gowatch
- Watch Go files.
- Run `gofmt` on saved files.
- Run main package go file that has `main` function
- Or use it with to watch any files and take whatever action you want.

## Install

If you already have your *Go* workspace and `GOPATH` set, simply run:

```
go get -u github.com/sikhapol/gowatch
```

## How to use

### Simple
If watching all `go` in current directory and all sub directories, then run `gofmt -w` on saving file. Then run `go run` on the main package source file with `main` function in it is what you want. Then simply run:

```
gowatch
```

### Specify watched directory

```
gowatch /path/to/watch/
```


## Format only

```
gowatch -n .
```

## For everything else

`gowatch` knows if you send input to another command with UNIX pipe.
In this case, it will just print the changed file names to `stdout` which you can then send to another command like `xargs` to further do anything interesting.

```
gowatch | xargs cat
```
