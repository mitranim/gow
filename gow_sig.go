package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/mitranim/gg"
)

/*
Internal tool for handling OS signals.
*/
type Sig struct {
	Mained
	Chan gg.Chan[os.Signal]
}

/*
This removes our custom handling of OS signals, falling back on the default
behavior of the Go runtime. This doesn't bother to stop the `(*Sig).Run`
goroutine. Terminating the entire `gow` process takes care of that.
*/
func (self *Sig) Deinit() {
	if self.Chan != nil {
		signal.Stop(self.Chan)
	}
}

/*
We override Go's default signal handling to ensure cleanup before exit.
Clean includes restoring the previous terminal state and broadcasting
kill signals to any descendant processes. Without this override, some
OS signals would kill us without allowing us to run cleanup.

The set of signals registered here MUST match the set of signals explicitly
handled by this program; see below.
*/
func (self *Sig) Init(main *Main) {
	self.Mained.Init(main)
	self.Chan.InitCap(1)
	signal.Notify(self.Chan, KILL_SIGS_OS...)
}

func (self *Sig) Run() {
	main := self.Main()

	for val := range self.Chan {
		// Should work on all Unix systems. At the time of writing,
		// we're not prepared to support other systems.
		sig := val.(syscall.Signal)

		if KILL_SIG_SET.Has(sig) {
			if main.Opt.Verb {
				log.Println(`received kill signal:`, sig)
			}
			main.Kill(sig)
			continue
		}

		if main.Opt.Verb {
			log.Println(`received unknown signal:`, sig)
		}
	}
}
