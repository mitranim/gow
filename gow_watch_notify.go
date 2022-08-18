package main

import (
	"path/filepath"

	"github.com/mitranim/gg"
	"github.com/rjeczalik/notify"
)

// Implementation of `Watcher` that uses "github.com/rjeczalik/notify".
type WatchNotify struct {
	Mained
	Done   gg.Chan[struct{}]
	Events gg.Chan[notify.EventInfo]
}

func (self *WatchNotify) Init(main *Main) {
	self.Mained.Init(main)
	self.Done.Init()
	self.Events.InitCap(1)

	paths := main.Opt.Watch
	verb := main.Opt.Verb && !gg.Equal(paths, OptDefault().Watch)

	for _, path := range paths {
		path = filepath.Join(path, `...`)
		if verb {
			log.Printf(`watching %q`, path)
		}
		gg.Try(notify.Watch(path, self.Events, notify.All))
	}
}

func (self *WatchNotify) Deinit() {
	self.Done.SendZeroOpt()
	if self.Events != nil {
		notify.Stop(self.Events)
	}
}

func (self WatchNotify) Run() {
	main := self.Main()

	for {
		select {
		case <-self.Done:
			return

		case event := <-self.Events:
			if main.Opt.ShouldRestart(event) {
				if main.Opt.Verb {
					log.Println(`restarting on FS event:`, event)
				}
				main.Restart()
			}
		}
	}
}
