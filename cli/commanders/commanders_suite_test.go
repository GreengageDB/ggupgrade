// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package commanders_test

import (
	"os"
	"testing"

	"github.com/GreengageDB/ggupgrade/testutils/exectest"
	"github.com/GreengageDB/ggupgrade/testutils/testlog"
)

func TestMain(m *testing.M) {
	testlog.SetupTestLogger()
	os.Exit(exectest.Run(m))
}
