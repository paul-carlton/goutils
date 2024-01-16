// Package kubectl executes various kubectl sub-commands in a forked shell
//
//go:generate mockgen -destination=../mocks/kubectl/mockExecProvider.go -package=mocks -source=execProvider.go . ExecProvider
package kubectl

// Copied from Kraan - https://github.com/fidelity/kraan
/*
	Copyright 2020 The Kraan contributors.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

		http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

import (
	"os"
	"os/exec"

	"github.com/paul-carlton/goutils/pkg/logging"
	"github.com/pkg/errors"
)

// ExecProvider interface defines functions Kubectl uses to verify and execute a local command.
type ExecProvider interface {
	FileExists(filePath string) bool
	FindOnPath(file string) (string, error)
	ExecCmd(name string, arg ...string) ([]byte, error)
}

// OsExecProvider implements the ExecProvider interface using the os/exec go module.
type realExecProvider struct{}

// NewExecProvider returns an instance of OsExecProvider to implement the ExecProvider interface.
func newExecProvider() ExecProvider {
	return realExecProvider{}
}

func (p realExecProvider) FileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}

func (p realExecProvider) FindOnPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (p realExecProvider) ExecCmd(name string, arg ...string) ([]byte, error) {
	errKubeCtl := errors.New("kubectl error")

	data, err := exec.Command(name, arg...).Output()
	if err != nil {
		exitError, ok := err.(*exec.ExitError)
		if ok {
			return nil, errors.Wrapf(errKubeCtl, "%s - kubectl failed, %s, error\n%s", logging.CallerStr(logging.Me), exitError.ProcessState.String(), string(exitError.Stderr))
		}

		return nil, err
	}

	return data, err
}
