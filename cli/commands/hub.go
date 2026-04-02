// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/GreengageDB/ggupgrade/config"
	"github.com/GreengageDB/ggupgrade/hub"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/upgrade"
	"github.com/GreengageDB/ggupgrade/utils"
	"github.com/GreengageDB/ggupgrade/utils/daemon"
	"github.com/GreengageDB/ggupgrade/utils/logger"
)

func Hub() *cobra.Command {
	var hubPort int
	var shouldDaemonize bool

	var cmd = &cobra.Command{
		Use:    "hub",
		Short:  "start the hub",
		Long:   "start the hub",
		Hidden: true,
		Args:   cobra.MaximumNArgs(0), //no positional args allowed
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Initialize("hub")
			defer logger.WritePanics()

			exist, err := upgrade.PathExist(utils.GetStateDir())
			if err != nil {
				return err
			}

			if !exist {
				nextAction := fmt.Sprintf(`Run "ggupgrade %s" to start the hub.`, idl.Step_initialize)
				err = fmt.Errorf("ggupgrade state directory %q does not exist", utils.GetStateDir())
				return utils.NewNextActionErr(err, nextAction)
			}

			conf, err := config.Read()
			if err != nil {
				return err
			}

			// allow command line args precedence over config file values
			if cmd.Flag("port").Changed {
				conf.HubPort = hubPort
			}

			hubServer := hub.New(conf)
			return hubServer.Start(conf.HubPort, shouldDaemonize)
		},
	}

	cmd.Flags().IntVar(&hubPort, "port", upgrade.DefaultHubPort, "the port to listen for commands on")

	daemon.MakeDaemonizable(cmd, &shouldDaemonize)

	return cmd
}
