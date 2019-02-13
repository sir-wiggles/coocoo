package main // import "github.com/sir-wiggles/coocoo"

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
)

type FlagList []string

func (t *FlagList) String() string {
	return "list of values"
}

func (t *FlagList) Set(value string) error {
	*t = append(*t, value)
	return nil
}

var (
	patterns FlagList
	command  string
)

func main() {

	flag.Var(&patterns, "p", "patterns that will trigger")
	flag.StringVar(&command, "c", "", "command to watch")

	flag.Parse()

	commandPieces := strings.Split(command, " ")

	ch := NewChangeHandler(patterns, commandPieces...)
	watcher, err := NewWatcher(".", ch)
	if err != nil {
		log.Fatal(err)
	}

	watcher.Watch()
}

type Watcher struct {
	*fsnotify.Watcher
	ticker        *time.Ticker
	changeHandler *ChangeHandler
}

func NewWatcher(directory string, handler *ChangeHandler) (*Watcher, error) {

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	walk(watcher, "CREATE")

	return &Watcher{
		watcher,
		time.NewTicker(100 * time.Millisecond),
		handler,
	}, nil
}

func (w Watcher) Watch() {
	for {
		select {
		case err, ok := <-w.Errors:
			log.Println("error:", err, ok)
			if !ok {
				return
			}
		case event := <-w.Events:
			log.Println(event)
			w.drain(event)
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
	w.changeHandler.Handle(changed)
}

type ChangeHandler struct {
	*exec.Cmd
	name     string
	args     []string
	triggers []*regexp.Regexp
}

func NewChangeHandler(patterns []string, command ...string) *ChangeHandler {

	h := &ChangeHandler{
		name:     command[0],
		args:     command[1:],
		triggers: compile(patterns),
	}

	h.Handle(nil)
	return h
}

func (c *ChangeHandler) Handle(changed map[string]fsnotify.Event) error {

	log.Println("changed", changed)
	if !c.tripped(changed) {
		return nil
	}

	if err := kill(c.Cmd); err != nil {
		log.Printf("Kill error: %s", err)
		return err
	}

	c.Cmd = exec.Command(c.name, c.args...)
	c.Cmd.Stdout = os.Stdout
	c.Cmd.Stderr = os.Stderr

	if err := c.Start(); err != nil {
		return err
	}
	color.Green("+pid %d", c.Process.Pid)
	return nil
}

func (c *ChangeHandler) tripped(changed map[string]fsnotify.Event) bool {

	// if we don't have any triggers then we'll watch everything
	if changed == nil || len(c.triggers) == 0 {
		return true
	}

	for file, _ := range changed {
		for _, trigger := range c.triggers {
			if trigger.MatchString(file) {
				return true
			}
		}
	}
	return false
}

func compile(patterns []string) []*regexp.Regexp {
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

func kill(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid == 0 {
		return nil
	}
	color.Red("-pid %d", cmd.Process.Pid)
	return cmd.Process.Kill()
}

func walk(watcher *fsnotify.Watcher, action string) error {

	return filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err

		} else if info.IsDir() && strings.HasPrefix(info.Name(), ".") && len(info.Name()) > 1 {
			color.Red("- %s", path)
			return filepath.SkipDir

		} else if info.IsDir() {
			return watcher.Add(path)
		}

		return nil
	})
}
