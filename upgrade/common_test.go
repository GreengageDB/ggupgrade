// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"os"
	"os/exec"
	"testing"

	"github.com/GreengageDB/ggupgrade/testutils/exectest"
)

func init() {
	ResetExecCommand()

	exectest.RegisterMains(
		Success,
		Failure,
	)
}

func Success() {}

func Failure() {
	_, _ = os.Stderr.WriteString(os.ErrPermission.Error())
	os.Exit(1)
}

var ExecCommand = exec.Command

func SetExecCommand(cmdFunc exectest.Command) {
	ExecCommand = cmdFunc
}

func ResetExecCommand() {
	ExecCommand = nil
}

func TestMain(m *testing.M) {
	os.Exit(exectest.Run(m))
}
