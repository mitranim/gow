## Overview

**Go** **W**atch: missing watch mode for the `go` command. It's invoked exactly like `go`, but also watches Go files and reruns on changes.

Currently requires Unix (MacOS, Linux, BSD). On Windows, runs under WSL.

## TOC

* [Overview](#overview)
* [Why](#why)
* [Installation](#installation)
* [Usage](#usage)
* [Hotkeys](#hotkeys)
* [Watching Templates](#watching-templates)
* [Alternatives](#alternatives)
* [License](#license)
* [Misc](#misc)

## Why

Why not other runners, general-purpose watchers, etc:

* Go-specific, easy to remember.
* Ignores non-Go files by default.
* Better watcher: no unnecessary delays, not even a split second; uses the excellent https://github.com/rjeczalik/notify.
* Silent by default.
* No garbage files.
* Can properly clear the terminal on restart.
* Has hotkeys!

## Installation

Make sure you have Go installed. Version 1.11 or higher is preferred.

```sh
go get -u github.com/mitranim/gow
```

This will download the source and compile the executable into `$GOPATH/bin/gow`. Make sure `$GOPATH/bin` is in your `$PATH` so the shell can discover the `gow` command. For example, my `~/.profile` contains this:

```sh
export GOPATH=~/go
export PATH=$PATH:$GOPATH/bin
```

Alternatively, you can run the executable using the full path. At the time of writing, `~/go` is the default `$GOPATH` for Go installations. Some systems may have a different one.

```sh
~/go/bin/gow
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

# Specify file extension to watch
gow -e=go,mod,html run .

# Help
gow -h
```

The first argument to `gow`, after the flags, can be any Go subcommand: `build`, `install`, `tool`, you name it.

## Hotkeys

Supported control codes with commonly associated hotkeys:

```
3     ^C    kill subprocess or self with SIGINT
18    ^R    kill subprocess with SIGTERM, restart
20    ^T    kill subprocess with SIGTERM
28    ^\    kill subprocess or self with SIGQUIT
31    ^?    print the currently running command
```

Other input is forwarded to the subprocess as-is.

## Watching Templates

Many Go programs, such as servers, include template files, and want to recompile those templates on change.

Easy but slow way: use `gow -e`.

```sh
gow -e=go,mod,html run .
```

This restarts your entire app on change to any `.html` file in the current directory. Beware: if the app also generates files with the same extensions, this could cause an infinite restart loop. Ignore any output directories with `-i`:

```sh
gow -e=go,mod,html -i=target run .
```

A smarter approach would be to watch the template files from _inside_ the app and recompile them without restarting the entire app. This is out of scope for `gow`.

## Alternatives

For general purpose file watching, consider these excellent tools:

  * https://github.com/emcrisostomo/fswatch
  * https://github.com/mattgreen/watchexec

## License

https://unlicense.org

## Misc

I'm receptive to suggestions. If this tool _almost_ satisfies you but needs changes, open an issue or chat me up. Contacts: https://mitranim.com/#contacts
