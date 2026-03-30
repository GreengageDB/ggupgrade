// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package commands

import "github.com/fatih/color"

var initializeConfirmationText = `
You are about to initialize a major-version upgrade of Greengage.

%s will carry out the following steps:
%s
ggupgrade log files can be found on all hosts in %s

ggupgrade initialize will use these values from %s
source_master_port:   %d
source_gphome:        %s
target_gphome:        %s
mode:                 %s
disk_free_ratio:      %.1f
pg_upgrade_jobs:      %d
use_hba_hostnames:    %t
dynamic_library_path: %s
temp_port_range:      %s
hub_port:             %d
agent_port:           %d

You will still have the opportunity to revert the cluster to its original state 
after this step.
` + color.RedString(`
WARNING: Do not perform operations on the cluster until ggupgrade is 
finalized or reverted.`) + `

Before proceeding, ensure the following have occurred:
 - Take a backup of the source Greengage cluster
 - Run gpcheckcat to ensure the source catalog has no inconsistencies
 - Run gpstate -e to ensure the source cluster's segments are up and in preferred roles
`

var executeConfirmationText = `
You are about to run the "execute" command for a major-version upgrade of Greengage.
This should be done only during a downtime window.
%s
%s will carry out the following steps:
%s
ggupgrade log files can be found on all hosts in %s

You will still have the opportunity to revert the cluster to its original state
after this step.
` + color.RedString(`
WARNING: Do not perform operations on the source cluster until ggupgrade is
finalized or reverted.
`)

var finalizeConfirmationText = `
You are about to finalize a major-version upgrade of Greengage.
This should be done only during a downtime window.

%s will carry out the following steps:
%s
ggupgrade log files can be found on all hosts in %s
` + color.RedString(`
WARNING: You will not be able to revert the cluster to its original state after this step.

WARNING: Do not perform operations on the source and target clusters until ggupgrade is 
finalized or reverted.
`)

var revertConfirmationText = `
You are about to revert this upgrade.
This should be done only during a downtime window.

%s will carry out the following steps:
%s
ggupgrade log files can be found on all hosts in %s
` + color.RedString(`
WARNING: You cannot revert if you do not have mirrors & standby configured, and execute has started.

WARNING: Do not perform operations on the source and target clusters until ggupgrade revert
has completed.
`)

var revertWarningText = color.RedString(`
WARNING
_______
The source cluster does not have standby and/or mirrors and 
is being upgraded in link mode.

Once "ggupgrade execute" has started, there will be no way to
return the cluster to its original state using "ggupgrade revert".

If you do not already have a backup, we strongly recommend that
you run "ggupgrade revert" now and take a backup of the cluster.
`)
