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

import "github.com/soluble-ai/soluble-cli/pkg/options"

type Interface interface {
	options.Interface
	GetToolOptions() *ToolOpts
	GetDirectoryBasedToolOptions() *DirectoryBasedToolOpts
	Validate() error
	Name() string
	IsNonAssessment() bool
}

// A Single tool runs and returns a single result
type Single interface {
	Interface
	Run() (*Result, error)
}

// A Consolidated tool runs and returns multiple results (typically by invoking
// other tools)
type Consolidated interface {
	Interface
	RunAll() (Results, error)
}
