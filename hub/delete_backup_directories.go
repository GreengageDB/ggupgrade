// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package hub

import (
	"context"

	"github.com/GreengageDB/ggupgrade/config/backupdir"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/step"
	"github.com/GreengageDB/ggupgrade/upgrade"
)

func DeleteBackupDirectories(streams step.OutStreams, agentConns []*idl.Connection, backupDirs backupdir.BackupDirs) error {
	err := upgrade.DeleteDirectories([]string{backupDirs.CoordinatorBackupDir}, []string{}, streams)
	if err != nil {
		return err
	}

	request := func(conn *idl.Connection) error {
		if _, ok := backupDirs.AgentHostsToBackupDir[conn.Hostname]; !ok {
			return nil
		}

		req := &idl.DeleteBackupDirectoryRequest{BackupDir: backupDirs.AgentHostsToBackupDir[conn.Hostname]}
		_, err := conn.AgentClient.DeleteBackupDirectory(context.Background(), req)
		return err
	}

	return ExecuteRPC(agentConns, request)
}
