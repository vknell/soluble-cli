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

package terrascan

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/soluble-ai/go-jnode"
	"github.com/soluble-ai/soluble-cli/pkg/assessments"
	"github.com/soluble-ai/soluble-cli/pkg/download"
	"github.com/soluble-ai/soluble-cli/pkg/tools"
	"github.com/soluble-ai/soluble-cli/pkg/util"
	"github.com/spf13/cobra"
)

var (
	_ tools.Single = &Tool{}
)

type Tool struct {
	tools.DirectoryBasedToolOpts
	PolicyType string
}

func (t *Tool) Name() string {
	return "terrascan"
}

func (t *Tool) Register(cmd *cobra.Command) {
	t.DirectoryBasedToolOpts.Register(cmd)
	cmd.Flags().StringVarP(&t.PolicyType, "policy-type", "t", "", "The `policy-type` (aws, azure, gcp, k8s).  Required unless using custom policies.")
}

func (t *Tool) Run() (*tools.Result, error) {
	args := []string{"scan", "-d", t.GetDirectory(), "-o", "json"}
	customPoliciesDir, err := t.GetCustomPoliciesDir()
	if err != nil {
		return nil, err
	}
	if customPoliciesDir != "" {
		args = append(args, "-p", customPoliciesDir)
	} else {
		if t.PolicyType == "" {
			return nil, fmt.Errorf("--policy-type must be given unless using custom policies")
		}
		args = append(args, "-t", t.PolicyType)
	}
	d, err := t.InstallTool(&download.Spec{
		URL: "github.com/accurics/terrascan",
	})
	if err != nil {
		return nil, err
	}
	program := filepath.Join(d.Dir, "terrascan")
	scan := exec.Command(program, args...)
	t.LogCommand(scan)
	scan.Stderr = os.Stderr
	output, err := scan.Output()
	if err != nil && util.ExitCode(err) != 3 {
		// terrascan exits with exit code 3 if violations were found
		return nil, err
	}
	n, err := jnode.FromJSON(output)
	if err != nil {
		return nil, err
	}
	result := t.parseResults(n)
	if d.Version != "" {
		result.AddValue("TERRASCAN_VERSION", d.Version)
	}
	return result, nil
}

func (t *Tool) parseResults(n *jnode.Node) *tools.Result {
	findings := assessments.Findings{}
	violations := n.Path("results").Path("violations")
	if violations.Size() > 0 {
		violations = util.RemoveJNodeElementsIf(violations, func(e *jnode.Node) bool {
			return t.IsExcluded(e.Path("file").AsText())
		})
		n.Path("results").Put("violations", violations)
		for _, v := range violations.Elements() {
			findings = append(findings, &assessments.Finding{
				FilePath:    v.Path("file").AsText(),
				Line:        v.Path("line").AsInt(),
				Description: v.Path("description").AsText(),
				Tool: map[string]string{
					"category": v.Path("category").AsText(),
					"rule_id":  v.Path("rule_id").AsText(),
					"severity": v.Path("severity").AsText(),
				},
			})
		}
	}
	result := &tools.Result{
		Data:      n,
		Directory: t.GetDirectory(),
		Findings:  findings,
	}
	return result
}
