package test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/soluble-ai/go-jnode"
	"github.com/soluble-ai/soluble-cli/cmd/root"
	"github.com/soluble-ai/soluble-cli/pkg/log"
	"github.com/soluble-ai/soluble-cli/pkg/util"
)

type Command struct {
	T    *testing.T
	Args []string
	Out  *bytes.Buffer
}

func NewCommand(t *testing.T, args ...string) *Command {
	return &Command{T: t, Args: args}
}

func (c *Command) Run() error {
	color.NoColor = true
	wd, err := os.Getwd()
	util.Must(err)
	log.Infof("Running command {primary:%s} {secondary:(in %s)}", strings.Join(c.Args, " "), wd)
	cmd := root.Command()
	cmd.SetArgs(c.Args)
	c.Out = &bytes.Buffer{}
	cmd.SetOut(c.Out)
	err = cmd.Execute()
	if err != nil {
		log.Errorf("{primary:%s} returned error - {danger:%s}", c.Args[0], err.Error())
	}
	return err
}

func (c *Command) JSON() *jnode.Node {
	n, jerr := jnode.FromJSON(c.Out.Bytes())
	if jerr != nil {
		log.Errorf("{primary:%s} did not return JSON - {danger:%s}", c.Args[0], jerr)
	}
	return n
}

func (c *Command) Must(err error) {
	c.T.Helper()
	if err != nil {
		c.T.Fatal(err)
	}
}
