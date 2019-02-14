package main

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/fatih/color"
)

type Command struct {
	*exec.Cmd
	name string
	args []string
}

func NewCommand(command ...string) (*Command, error) {

	c := &Command{
		name: command[0],
		args: command[1:],
	}

	return c, c.Run()
}

func (c *Command) Restart() *Command {

	if err := c.kill(); err != nil {
		return c
	}
	c.Run()
	return c
}

func (c Command) Run() error {
	c.Cmd = exec.Command(c.name, c.args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := c.Start(); err != nil {
		return err
	}
	color.Green("+pid %d", c.Process.Pid)
	return nil
}

func (c Command) kill() error {
	if c.Cmd == nil || c.Process == nil || c.Process.Pid == 0 {
		return nil
	}

	color.Red("-pid %d", c.Process.Pid)
	return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
}
