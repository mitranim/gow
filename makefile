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
# by the rule name. This is equivalent to marking every rule with `.PHONY`,
# making our makefile cleaner. In projects where some rule names are patterns
# for artifact paths, this should be removed, and abstract rules should be
# explicitly marked with `.PHONY`.
MAKEFLAGS := --silent --always-make

MAKE_CONC := $(MAKE) -j 128 --makefile $(lastword $(MAKEFILE_LIST))
VERB := $(if $(filter $(verb),false),,-v)
CLEAR := $(if $(filter $(clear),false),,-c)
GO_SRC := .
GO_PKG := ./$(or $(pkg),$(GO_SRC)/...)
GO_FLAGS := -tags=$(tags) -mod=mod
GO_RUN_ARGS := $(GO_FLAGS) $(GO_SRC) $(run)
GO_TEST_FAIL := $(if $(filter $(fail),false),,-failfast)
GO_TEST_SHORT := $(if $(filter $(short),true), -short,)
GO_TEST_FLAGS := -count=1 $(GO_FLAGS) $(VERB) $(GO_TEST_FAIL) $(GO_TEST_SHORT)
GO_TEST_PATTERNS := -run="$(run)"
GO_TEST_ARGS := $(GO_PKG) $(GO_TEST_FLAGS) $(GO_TEST_PATTERNS)

# Expects a "stable" version of `gow` to be installed globally.
GOW := gow $(CLEAR) $(VERB) -r=false

watch:
	$(MAKE_CONC) test.w vet.w

test.w:
	$(GOW) test $(GO_TEST_ARGS)

test:
	go test $(GO_TEST_ARGS)

vet.w:
	$(GOW) vet $(GO_FLAGS)

vet:
	go vet $(GO_FLAGS)

run.w:
	$(GOW) run $(GO_RUN_ARGS)

run:
	go run $(GO_RUN_ARGS)
