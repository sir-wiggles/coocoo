package main // import "github.com/sir-wiggles/coocoo"

import (
	"flag"
	"log"
	"strings"
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

	cmd, err := NewCommand(commandPieces...)
	if err != nil {
		log.Fatal(err)
	}

	watcher, err := NewWatcher(cmd)
	if err != nil {
		log.Fatal(err)
	}

	watcher.Directories([]string{"."})
	watcher.Patterns(patterns)

	watcher.Watch()

}
