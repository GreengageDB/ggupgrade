// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"github.com/GreengageDB/ggupgrade/cli/commands"
	"github.com/GreengageDB/ggupgrade/utils"
	"github.com/GreengageDB/ggupgrade/utils/daemon"
	"github.com/GreengageDB/ggupgrade/utils/logger"
)

func main() {
	debug.SetTraceback("all")

	logger.Initialize("cli")
	defer logger.WritePanics()

	root := commands.BuildRootCommand()
	err := root.Execute()
	if err != nil && err != daemon.ErrSuccessfullyDaemonized {
		// We use gplog.Debug instead of Error so the error is not displayed
		// twice to the user in the terminal.
		log.Printf("%+v", err)

		// Print any additional actions that should be taken by the user.
		var actions utils.NextActionErr
		if errors.As(err, &actions) {
			fmt.Print(actions.Help())
		}

		os.Exit(1)
	}
}
