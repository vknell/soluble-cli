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

package xcp

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/soluble-ai/go-jnode"
	"github.com/soluble-ai/soluble-cli/pkg/api"
)

var metadataCommands = map[string]string{
	"SOLUBLE_METADATA_GIT_BRANCH":       "git rev-parse --abbrev-ref HEAD",
	"SOLUBLE_METADATA_GIT_COMMIT":       "git rev-parse HEAD",
	"SOLUBLE_METADATA_GIT_COMMIT_SHORT": "git rev-parse --short HEAD",
	"SOLUBLE_METADATA_GIT_DESCRIBE":     "git describe --tags --always",
	"SOLUBLE_METADATA_GIT_REMOTE":       "git ls-remote --get-url",
}

var (
	// We explicitly exclude a few keys due to their sensitive values.
	// The substrings below will cause the environment variable to be
	// skipped (not recorded).
	substringOmitEnv = []string{
		"SECRET", "KEY", "PRIVATE", "PASSWORD",
		"PASSPHRASE", "CREDS", "TOKEN", "AUTH",
		"ENC", "JWT",
		"_USR", "_PSW", // Jenkins credentials()
	}

	// While we perform the redactions based on substrings above,
	// we also maintain a list of known-sensitive keys to ensure
	// that we never capture these. Unlike above, these are an
	// exact match and not a substring match.
	explicitOmitEnv = []string{
		"BUILDKITE_S3_SECRET_ACCESS_KEY", // Buildkite
		"BUILDKITE_S3_ACCESS_KEY_ID",     // Buildkite
		"BUILDKITE_S3_ACCESS_URL",        // Buildkite
		"BUILDKITE_COMMAND",              // Buildkite
		"BUILDKITE_SCRIPT_PATH",          // Buildkite
		"KEY",                            // CircleCI encrypted-files decryption key
		"CI_DEPLOY_PASSWORD",             // Gitlab
		"CI_DEPLOY_USER",                 // Gitlab
		"CI_JOB_TOKEN",                   // Gitlab
		"CI_JOB_JWT",                     // Gitlab
		"CI_REGISTRY_USER",               // Gitlab
		"CI_REGISTRY_PASSWORD",           // Gitlab
		"CI_REGISTRY_USER",               // Gitlab
	}
)

// Include CI-related environment variables in the request.
func WithCIEnv(dir string) api.Option {
	return func(req *resty.Request) {
		if req.Method == "GET" {
			req.SetQueryParams(GetCIEnv(dir))
		} else {
			req.SetMultipartFormData(GetCIEnv(dir))
		}
	}
}

// Include CI-related information in the body of a request
func WithCIEnvBody(dir string) api.Option {
	return func(r *resty.Request) {
		body := jnode.NewObjectNode()
		for k, v := range GetCIEnv(dir) {
			body.Put(k, v)
		}
		r.SetBody(body)
	}
}

// For XCPPost, include a file from a reader.
func WithFileFromReader(param, filename string, reader io.Reader) api.Option {
	return func(req *resty.Request) {
		req.SetFileReader(param, filename, reader)
	}
}

func GetCIEnv(dir string) map[string]string {
	dir = filepath.Clean(dir)
	values := map[string]string{}
	allEnvs := make(map[string]string)
	for _, e := range os.Environ() {
		split := strings.Split(e, "=")
		allEnvs[split[0]] = split[1]
	}
	var ciSystem string
	// We don't want all of the environment variables, however.
envLoop:
	for k, v := range allEnvs {
		k = strings.ToUpper(k)
		for _, s := range substringOmitEnv {
			if strings.Contains(k, s) {
				continue envLoop
			}
		}
		for _, s := range explicitOmitEnv {
			if k == s {
				continue envLoop
			}
		}

		// If the key has made it through the filtering above and is
		// from a CI system, we include it.
		if strings.HasPrefix(k, "GITHUB_") ||
			strings.HasPrefix(k, "CIRCLE_") ||
			strings.HasPrefix(k, "GITLAB_") ||
			strings.HasPrefix(k, "CI_") ||
			strings.HasPrefix(k, "BUILDKITE_") ||
			strings.HasPrefix(k, "ZODIAC_") {
			values[k] = v

			// and if we haven't set a CI system yet, set it
			if ciSystem == "" {
				idx := strings.Index(k, "_")
				if idx > 0 {
					ciSystem = k[:idx]
				}
			}
		}
	}
	values["SOLUBLE_METADATA_CI_SYSTEM"] = ciSystem

	// evaluate the "easy" metadata commands
	for k, command := range metadataCommands {
		argv := strings.Split(command, " ")
		// #nosec G204
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err == nil {
			values[k] = strings.TrimSpace(string(out))
		}
	}
	if s := normalizeGitRemote(values["SOLUBLE_METADATA_GIT_REMOTE"]); s != "" {
		values["SOLUBLE_METADATA_GIT_REMOTE"] = s
	}

	// Hostname
	h, err := os.Hostname()
	if err == nil {
		values["SOLUBLE_METADATA_HOSTNAME"] = h
	}

	return values
}

func normalizeGitRemote(s string) string {
	// transform "git@github.com:fizz/buzz.git" to "github.com/fizz/buzz"
	at := strings.Index(s, "@")
	dotgit := strings.LastIndex(s, ".git")
	if at > 0 && dotgit > 0 {
		return strings.Replace(s[at+1:dotgit], ":", "/", 1)
	}
	return s
}
