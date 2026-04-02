// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package hub

import (
	"log"
	"strconv"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/step"
)

// UpgradeStandby removes any possible existing standby from the cluster
// before adding a new one for idempotency. In the happy-path, we expect this to
// fail as there should not be an existing  standby for the cluster.
func UpgradeStandby(streams step.OutStreams, intermediate *greengage.Cluster, useHbaHostnames bool) error {
	err := intermediate.RunGreengageCmd(streams, "gpinitstandby", "-r", "-a")
	if err != nil {
		// FIXME: Don't ignore actual errors. Perhaps check if there is a standby to remove before attemtping.
		log.Printf("Failed to remove existing standby. Expected during normal operation. %v", err)
	}

	args := []string{
		"-P", strconv.Itoa(intermediate.Standby().Port),
		"-s", intermediate.Standby().Hostname,
		"-S", intermediate.Standby().DataDir,
		"-a",
	}

	if useHbaHostnames {
		args = append(args, "--hba-hostnames")
	}

	return intermediate.RunGreengageCmd(streams, "gpinitstandby", args...)
}
