## Overview

Watch Go files and execute a Go command like `go run` or `go vet`. Expects Go 1.11 or higher.

Works on MacOS, should work on Linux. Pull requests for Windows are welcome.

Replaces https://github.com/Mitranim/gorun, which has become obsolete since Go 1.11.

## Why

Why not other runners, general-purpose watchers, etc:

  * Go-specific: easy to remember, ignores non-Go files
  * better watcher: no unnecessary delays, not even a split second; uses the excellent https://github.com/rjeczalik/notify
  * silent
  * no garbage files

## Installation

Make sure you have Go installed. Version 1.11 or higher is preferred.

```sh
go get -u github.com/Mitranim/gow
```

This will download the source and compile the executable: `$GOPATH/bin/gow`. Make sure `$GOPATH/bin` is in your `$PATH` so the shell can discover it. For example, my `~/.profile` contains this:

```sh
export GOPATH=~/go
export PATH=$PATH:$GOPATH/bin
```

## Usage

```sh
# Start and restart on change
gow run .

# Pass args to the program
gow run . arg0 arg1 ...

# Run subdirectory
gow run ./go

# Vet and re-vet on change
gow vet

# Help
gow -h
```

The first argument to `gow` can be any Go subcommand: `build`, `install`, `tool`, you name it.

## Alternatives

For general purpose file watching, consider these excellent tools:

  * https://github.com/emcrisostomo/fswatch
  * https://github.com/mattgreen/watchexec

## License

https://en.wikipedia.org/wiki/WTFPL

## Misc

I'm receptive to suggestions. If this tool _almost_ satisfies you but needs changes, open an issue or chat me up. Contacts: https://mitranim.com/#contacts
