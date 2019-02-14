package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	*fsnotify.Watcher
	ticker      *time.Ticker
	directories map[string]bool
	triggers    []*regexp.Regexp
	command     *Command
}

func NewWatcher(command *Command) (*Watcher, error) {

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		Watcher:     watcher,
		ticker:      time.NewTicker(100 * time.Millisecond),
		directories: make(map[string]bool),
		command:     command,
	}

	return w, nil
}

func (w Watcher) Directories(directories []string) {
	for _, directory := range directories {
		w.walk(directory, "CREATE")
	}
}

func (w Watcher) Patterns(patterns []string) []*regexp.Regexp {
	var triggers = make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if compiled, err := regexp.Compile(pattern); err != nil {
			log.Printf("trigger pattern compile error: %s", err)
		} else {
			triggers = append(triggers, compiled)
		}
	}
	return triggers
}

func (w Watcher) Watch() {

	sig := make(chan os.Signal, 10)
	signal.Notify(sig, os.Interrupt)

	for {
		select {
		case err, ok := <-w.Errors:
			log.Println("fsnotify.Watcher.Errors: ", err)
			if !ok {
				return
			}
		case event := <-w.Events:
			w.drain(event)
		case <-sig:
			w.command.kill()
			return
		}
	}
}

func (w Watcher) drain(event fsnotify.Event) {
	var (
		changed     = map[string]fsnotify.Event{event.Name: event}
		ticks       = 0
		ticksNeeded = 2
	)

	for {
		select {
		case event := <-w.Events:
			if ticks <= ticksNeeded-1 && ticks >= 0 {
				ticks--
			}
			changed[event.Name] = event

		case <-w.ticker.C:
			ticks++
			if ticks == ticksNeeded {
				goto breakfor
			}
		}
	}

breakfor:
	w.changes(changed)
}

func (w *Watcher) tripped(changed map[string]fsnotify.Event) bool {

	// if we don't have any triggers then we'll watch everything
	if changed == nil || len(w.triggers) == 0 {
		for file, event := range changed {
			w.walk(file, event.Op.String())
		}
		return true
	}

	for file, event := range changed {
		w.walk(file, event.Op.String())
		for _, trigger := range w.triggers {
			if trigger.MatchString(file) {
				return true
			}
		}
	}
	return false
}

func (w *Watcher) changes(changed map[string]fsnotify.Event) error {

	if !w.tripped(changed) {
		return nil
	}

	w.command = w.command.Restart()
	return nil
}

func (w Watcher) walk(directory, action string) error {

	return filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			delete(w.directories, path)
			return nil

		} else if info.IsDir() && strings.HasPrefix(info.Name(), ".") && len(info.Name()) > 1 {
			color.Yellow("? %s", path)
			return filepath.SkipDir
		}

		switch action {
		case "CREATE":
			fileinfo, err := os.Stat(path)
			if err != nil {
				break
			}
			if fileinfo.IsDir() {
				w.directories[path] = true
				w.Add(path)
			}
		case "RENAME":
		case "REMOVE":
			w.Remove(path)
			delete(w.directories, path)
		default:
		}
		return nil

	})
}
