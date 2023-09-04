## Overview

**Go** **W**atch: missing watch mode for the `go` command. It's invoked exactly like `go`, but also watches Go files and reruns on changes.

Currently requires Unix (MacOS, Linux, BSD). On Windows, runs under WSL.

## TOC

* [Overview](#overview)
* [Why](#why)
* [Installation](#installation)
* [Usage](#usage)
* [Hotkeys](#hotkeys)
* [Gotchas](#gotchas)
* [Watching Templates](#watching-templates)
* [Alternatives](#alternatives)
* [License](#license)
* [Misc](#misc)

## Why

Why not other runners, general-purpose watchers, etc:

* Has hotkeys, such as `ctrl+r` to restart!
* Go-specific, easy to remember.
* Ignores non-Go files by default.
* Better watcher: recursive, no delays, no polling; uses https://github.com/rjeczalik/notify.
* Silent by default.
* No garbage files.
* Can properly clear the terminal on restart.
* Does not leak subprocesses.
* Minimal dependencies.

## Installation

Make sure you have Go installed, then run this:

```sh
go install github.com/mitranim/gow@latest
```

This should download the source and compile the executable into `$GOPATH/bin/gow`. Make sure `$GOPATH/bin` is in your `$PATH` so the shell can discover the `gow` command. For example, my `~/.profile` contains this:

```sh
export GOPATH="$HOME/go"
export PATH="$GOPATH/bin:$PATH"
```

Alternatively, you can run the executable using the full path. At the time of writing, `~/go` is the default `$GOPATH` for Go installations. Some systems may have a different one.

```sh
~/go/bin/gow
```

On MacOS, if installation fails with dylib-related errors, you may need to run `xcode-select --install` or install Xcode. This is caused by `gow`'s dependencies, which depend on C. See [#15](https://github.com/mitranim/gow/issues/15).

## Usage

The first argument to `gow`, after the flags, can be any Go subcommand: `build`, `install`, `tool`, you name it.

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

## Hotkeys

Supported control codes with commonly associated hotkeys. Exact keys may vary between terminal apps. For example, `^-` in MacOS Terminal vs `^?` in iTerm2.

```
3     ^C          Kill subprocess with SIGINT.
18    ^R          Kill subprocess with SIGTERM, restart.
20    ^T          Kill subprocess with SIGTERM.
28    ^\          Kill subprocess with SIGQUIT.
31    ^- or ^?    Print currently running command.
8     ^H          Print help.
127   ^H (MacOS)  Print help.
```

In slightly more technical terms, `gow` switches the terminal into [raw mode](https://en.wikibooks.org/wiki/Serial_Programming/termios), reads from stdin, interprets some ASCII control codes, and forwards the other input to the subprocess as-is. In raw mode, pressing one of these hotkeys causes a terminal to write the corresponding byte to stdin, which is then interpreted by `gow`. This can be disabled with `-r=false`.

## Gotchas

By default, `gow` expects to be a foreground process in an interactive terminal. When running `gow` as a background process, in Docker, or in any other non-interactive environment, you may see errors related to terminal state. To avoid the problem, run `gow` with `-r=false`. This also disables hotkey support. Examples of such errors:

```
> unable to read terminal state
> inappropriate ioctl for device
> operation not supported by device
```

## Watching Templates

Many Go programs, such as servers, include template files, and want to recompile those templates on change.

Easy but slow way: use `gow -e`.

```sh
gow -e=go,mod,html run .
```

This restarts your entire app on change to any `.html` file in the current directory or sub-directories. Beware: if the app also generates files with the same extensions, this could cause an infinite restart loop. Ignore any output directories with `-i`:

```sh
gow -e=go,mod,html -i=target run .
```

A smarter approach would be to watch the template files from _inside_ the app and recompile them without restarting the entire app. This is out of scope for `gow`.

Finally, you can use a pure-Go rendering system such as [github.com/mitranim/gax](https://github.com/mitranim/gax).

## Alternatives

For general purpose file watching, consider these excellent tools:

  * https://github.com/mattgreen/watchexec
  * https://github.com/emcrisostomo/fswatch

## License

https://unlicense.org

## Misc

I'm receptive to suggestions. If this tool _almost_ satisfies you but needs changes, open an issue or chat me up. Contacts: https://mitranim.com/#contacts
