## Overview

"gow" is the missing watch mode for the "go" command. It's invoked exactly like `go`, but also watches Go files and reruns on changes. Works on MacOS, should work on Linux. Pull requests for Windows are welcome.

## Why

Why not other runners, general-purpose watchers, etc:

  * Go-specific, easy to remember
  * ignores non-Go files by default
  * better watcher: no unnecessary delays, not even a split second; uses the excellent https://github.com/rjeczalik/notify
  * silent by default
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
gow run ./subdir

# Vet and re-vet on change; verbose mode is recommended
gow -v vet

# Clear terminal on restart
gow -c run .

# Help
gow -h
```

The first argument to `gow` can be any Go subcommand: `build`, `install`, `tool`, you name it.

## Control Keys

Supported control codes with commonly associated hotkeys:

```
3     ^C    kill subprocess or self with SIGINT
18    ^R    kill subprocess with SIGTERM, restart
20    ^T    kill subprocess with SIGTERM
28    ^\    kill subprocess or self with SIGQUIT
31    ^?    print the currently running command
```

Other input is forwarded to the subprocess as-is.

## Alternatives

For general purpose file watching, consider these excellent tools:

  * https://github.com/emcrisostomo/fswatch
  * https://github.com/mattgreen/watchexec

## License

https://en.wikipedia.org/wiki/WTFPL

## Misc

I'm receptive to suggestions. If this tool _almost_ satisfies you but needs changes, open an issue or chat me up. Contacts: https://mitranim.com/#contacts
