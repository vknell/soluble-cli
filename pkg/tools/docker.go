// Copyright 2021 Soluble Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tools

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/soluble-ai/soluble-cli/pkg/log"
)

type DockerError string

type DockerTool struct {
	Name                string
	Image               string
	DockerArgs          []string
	Args                []string
	DefaultNoDockerName string
	PolicyDirectory     string
	Stdout              io.Writer
	Stderr              io.Writer
	Directory           string
}

func (d DockerError) Error() string {
	return string(d)
}

func (d DockerError) Is(err error) bool {
	_, ok := err.(DockerError)
	return ok
}

func IsDockerError(err error) bool {
	return errors.Is(err, DockerError(""))
}

func hasDocker(options ...func(*exec.Cmd)) error {
	// "The operating-system independent way to check whether Docker is running
	// is to ask Docker, using the docker info command."
	// ref: https://docs.docker.com/config/daemon/#check-whether-docker-is-running
	c := exec.Command("docker", "info")
	for _, opt := range options {
		opt(c)
	}
	err := c.Run()
	switch c.ProcessState.ExitCode() {
	case 0:
		return nil
	case 1:
		log.Errorf("This command requires {primary:docker} but {danger:docker is not running}")
		return DockerError("the docker server is not running")
	case 127:
		log.Errorf("This command requires {primary:docker} but {danger:the docker command is not found}")
		return DockerError("the docker executable is not present, or is not in the PATH")
	default:
		log.Errorf("This command requires {primary:docker} but {danger:%s}", err)
		return DockerError("unknown error checking docker availability")
	}
}

func (t *DockerTool) run(skipPull bool) ([]byte, error) {
	if err := hasDocker(); err != nil {
		return nil, err
	}
	if !skipPull {
		// #nosec G204
		pull := exec.Command("docker", "pull", t.Image)
		out, err := pull.Output()
		if err != nil {
			os.Stderr.Write(out)
			log.Warnf("docker pull {primary:%s} failed: {warning:%s}", t.Image, err)
		}
	}
	args := t.getArgs(os.Getenv)
	run := exec.Command("docker", args...)
	log.Infof("Running {primary:%s}", strings.Join(run.Args, " "))
	run.Stdin = os.Stdin
	run.Stderr = os.Stderr
	if t.Stderr != nil {
		run.Stderr = t.Stderr
	}
	if t.Stdout != nil {
		run.Stdout = t.Stdout
		return nil, run.Run()
	}
	return run.Output()
}

func (t *DockerTool) getArgs(getenv func(string) string) []string {
	args := []string{"run", "--rm"}
	if t.Directory != "" {
		args = append(args, "-v", fmt.Sprintf("%s:/src", t.Directory),
			"-w", "/src")
	}
	if t.PolicyDirectory != "" {
		// mount the policy directory and rewrite args
		args = append(args, "-v", fmt.Sprintf("%s:/policy", t.PolicyDirectory))
		for i := range t.Args {
			if t.Args[i] == t.PolicyDirectory {
				t.Args[i] = "/policy"
			}
		}
	}
	args = append(args, t.DockerArgs...)
	args = appendProxyEnv(getenv, args)
	args = append(args, t.Image)
	args = append(args, t.Args...)
	return args
}

func appendProxyEnv(getenv func(string) string, args []string) []string {
	for _, k := range []string{
		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY",
	} {
		if v := getenv(k); v != "" {
			args = append(args, "-e", k)
		}
		lk := strings.ToLower(k)
		if v := getenv(lk); v != "" {
			args = append(args, "-e", lk)
		}
	}
	return args
}
