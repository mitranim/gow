# README
#
# This file is intended as an example for users of `gow`. It was adapted from a
# larger Go project. At the time of writing, `gow` does not support its own
# configuration files. For complex use cases, users are expected to use Make or
# another similar tool.
#
# Despite being primarily an example, this file contains actual working rules
# convenient for hacking on `gow`.

# This variable `MAKEFLAGS` is special: it modifies Make's own behaviors.
#
# `--silent` causes Make to execute rules without additional verbose logging.
# Without this, we would have to prefix each line in each rule with `@` to
# suppress logging.
#
# `--always-make` makes all rules "abstract". It causes Make to execute each
# rule without checking for an existing file matching the pattern represented
# by the rule name. This is equivalent to marking every rule with `.PHONY`, but
# keeps our makefile cleaner. In projects where some rule names are file names
# or artifact path patterns, this should be removed, and abstract rules should
# be explicitly marked with `.PHONY`.
MAKEFLAGS := --silent --always-make

# Shortcut for executing rules concurrently. See usage examples below.
MAKE_CONC := $(MAKE) -j 128 CONC=true clear=$(or $(clear),false)

VERB ?= $(if $(filter false,$(verb)),,-v)
CLEAR ?= $(if $(filter false,$(clear)),,-c)
GO_SRC ?= .
GO_PKG ?= ./$(or $(pkg),$(GO_SRC)/...)
GO_FLAGS ?= -tags=$(tags) -mod=mod
GO_RUN_ARGS ?= $(GO_FLAGS) $(GO_SRC) $(run)
GO_TEST_FAIL ?= $(if $(filter false,$(fail)),,-failfast)
GO_TEST_SHORT ?= $(if $(filter true,$(short)), -short,)
GO_TEST_FLAGS ?= -count=1 $(GO_FLAGS) $(VERB) $(GO_TEST_FAIL) $(GO_TEST_SHORT)
GO_TEST_PATTERNS ?= -run="$(run)"
GO_TEST_ARGS ?= $(GO_PKG) $(GO_TEST_FLAGS) $(GO_TEST_PATTERNS)
IS_TTY ?= $(shell test -t 0 && printf " ")

# Only one `gow` per terminal is allowed to use raw mode.
# Otherwise they conflict with each other.
GOW_HOTKEYS ?= -r=$(if $(filter true,$(CONC)),,$(if $(IS_TTY),true,false))

GOW_FLAGS ?= $(CLEAR) $(VERB) $(GOW_HOTKEYS)

# Expects an existing stable version of `gow`.
GOW ?= gow $(GOW_FLAGS)

watch:
	$(MAKE_CONC) dev_test_w dev_vet_w

# If everything works properly, then we should see a message about the FS event
# (file change), and tests should rerun. And, if everything _really_ works
# properly, modifying local files should trigger FS events in the container,
# causing `gow` to restart the test.
watch_linux:
	podman run --rm -it -v=$(PWD):/gow -w=/gow golang:alpine sleep 3 && echo 'package main' > touched.go & go run . -v test -v

all:
	$(MAKE_CONC) test vet

dev_test_w:
	go run $(GO_RUN_ARGS) $(GOW_FLAGS) test $(GO_TEST_FLAGS)

test_w:
	$(GOW) test $(GO_TEST_ARGS)

test:
	go test $(GO_TEST_ARGS)

dev_vet_w:
	go run $(GO_RUN_ARGS) $(GOW_FLAGS) vet $(GO_FLAGS)

vet_w:
	$(GOW) vet $(GO_FLAGS)

vet:
	go vet $(GO_FLAGS)

check:
	gopls check *.go

run_w:
	$(GOW) run $(GO_RUN_ARGS)

run:
	go run $(GO_RUN_ARGS)

install:
	go install $(GO_FLAGS) $(GO_SRC)

# This uses another watcher because if we're repeatedly reinstalling `gow`,
# then it's because we're experimenting and it's probably broken.
install_w:
	watchexec -n -r -d=1ms -- go install $(GO_FLAGS) $(GO_SRC)
