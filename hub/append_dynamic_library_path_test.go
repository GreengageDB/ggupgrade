// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package hub_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/hub"
	"github.com/GreengageDB/ggupgrade/testutils/exectest"
	"github.com/GreengageDB/ggupgrade/testutils/testlog"
)

func TestAppendDynamicLibraryPath(t *testing.T) {
	testlog.SetupTestLogger()

	intermediate := hub.MustCreateCluster(t, greengage.SegConfigs{
		{DbID: 1, ContentID: -1, Hostname: "coordinator", DataDir: "/data/qddir/seg.HqtFHX54y0o.-1", Port: 50432, Role: greengage.PrimaryRole},
		{DbID: 2, ContentID: -1, Hostname: "standby", DataDir: "/data/standby.HqtFHX54y0o", Port: 50433, Role: greengage.MirrorRole},
		{DbID: 3, ContentID: 0, Hostname: "sdw1", DataDir: "/data/dbfast1/seg.HqtFHX54y0o.1", Port: 50434, Role: greengage.PrimaryRole},
		{DbID: 4, ContentID: 0, Hostname: "sdw2", DataDir: "/data/dbfast_mirror1/seg.HqtFHX54y0o.1", Port: 50435, Role: greengage.MirrorRole},
		{DbID: 5, ContentID: 1, Hostname: "sdw2", DataDir: "/data/dbfast2/seg.HqtFHX54y0o.2", Port: 50436, Role: greengage.PrimaryRole},
		{DbID: 6, ContentID: 1, Hostname: "sdw1", DataDir: "/data/dbfast_mirror2/seg.HqtFHX54y0o.2", Port: 50437, Role: greengage.MirrorRole},
	})

	t.Run("returns error when gpconfig fails", func(t *testing.T) {
		greengage.SetGreengageCommand(exectest.NewCommand(hub.Failure))
		defer greengage.ResetGreengageCommand()

		err := hub.AppendDynamicLibraryPath(intermediate, "")
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("got error %#v want %T", err, exitErr)
		}

		if exitErr.ExitCode() != 1 {
			t.Errorf("got exit code %d want 1", exitErr.ExitCode())
		}
	})

	t.Run("returns error when gpconfig returns no value for dynamic_library_path", func(t *testing.T) {
		greengage.SetGreengageCommand(exectest.NewCommand(hub.Success))
		defer greengage.ResetGreengageCommand()

		err := hub.AppendDynamicLibraryPath(intermediate, "")
		expected := "issing value for dynamic_library_path"
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("got %+v, want %+v", err, expected)
		}
	})
}
