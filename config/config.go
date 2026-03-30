// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/xerrors"

	"github.com/GreengageDB/ggupgrade/config/backupdir"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/upgrade"
	"github.com/GreengageDB/ggupgrade/utils"
)

const ConfigFileName = "config.json"

type Config struct {
	// We do not combine the state directory and backup directory for
	// several reasons:
	// - The backup directory needs to be configurable since there
	// may not be enough space in the default location. If the state and
	// backup directories are combined and the backup directory needs to be
	// changed, then we have to preserve ggupgrade state by copying
	// substeps.json and config.json to the new location. This is awkward,
	// hard to manage, and error prone.
	// - The default state directory $HOME/.ggupgrade is known upfront with
	// no dependencies. Whereas the default backup directory is based on the
	// data directories. Having a state directory with no dependencies is
	// much easier to create and remove during the ggupgrade lifecycle.
	BackupDirs backupdir.BackupDirs

	// Source is the GPDB cluster that is being upgraded. It is populated during
	// the generation of the cluster config in the initialize step; before that,
	// it is nil.
	Source *greengage.Cluster

	// Intermediate represents the initialized target cluster that is upgraded
	// based on the source.
	Intermediate *greengage.Cluster

	// Target is the upgraded GPDB cluster. It is populated during the target
	// gpinitsystem execution in the initialize step; before that, it is nil.
	Target *greengage.Cluster

	HubPort         int
	AgentPort       int
	Mode            idl.Mode
	UseHbaHostnames bool
	UpgradeID       string
	PgUpgradeJobs   uint
}

func (conf *Config) Write() error {
	contents, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return xerrors.Errorf("marshal configuration file: %w", err)
	}

	return utils.AtomicallyWrite(GetConfigFile(), contents)
}

func Read() (*Config, error) {
	contents, err := os.ReadFile(GetConfigFile())
	if err != nil {
		return nil, err
	}

	conf := &Config{}
	err = json.Unmarshal(contents, &conf)
	if err != nil {
		return nil, xerrors.Errorf("unmarshal configuration file: %w", err)
	}

	return conf, nil
}

func GetConfigFile() string {
	return filepath.Join(utils.GetStateDir(), ConfigFileName)
}

func Create(db *sql.DB, hubPort int, agentPort int, sourceGPHome string, targetGPHome string, mode idl.Mode, useHbaHostnames bool, ports []int, pgUpgradeJobs uint, parentBackupDirs string) (Config, error) {
	source, err := greengage.ClusterFromDB(db, sourceGPHome, idl.ClusterDestination_source)
	if err != nil {
		return Config{}, xerrors.Errorf("retrieve source configuration: %w", err)
	}

	// Ensure segments are up, synchronized, and in their preferred role before proceeding.
	err = greengage.WaitForSegments(db, 5*time.Minute, &source)
	if err != nil {
		return Config{}, err
	}

	targetVersion, err := greengage.Version(targetGPHome)
	if err != nil {
		return Config{}, err
	}

	config := Config{}
	config.HubPort = hubPort
	config.AgentPort = agentPort
	config.Mode = mode
	config.UseHbaHostnames = useHbaHostnames
	config.UpgradeID = upgrade.NewID()
	config.PgUpgradeJobs = pgUpgradeJobs
	config.BackupDirs, err = backupdir.ParseParentBackupDirs(parentBackupDirs, source)
	if err != nil {
		return Config{}, err
	}

	target := source // create target cluster based off source cluster
	config.Source = &source

	config.Target = &target
	config.Target.Destination = idl.ClusterDestination_target
	config.Target.GPHome = targetGPHome
	config.Target.Version = targetVersion

	config.Intermediate, err = GenerateIntermediateCluster(config.Source, ports, config.UpgradeID, config.Target.Version, config.Target.GPHome)
	if err != nil {
		return Config{}, err
	}

	if err := EnsureTempPortRangeDoesNotOverlapWithSourceClusterPorts(config.Source, config.Intermediate); err != nil {
		return Config{}, err
	}

	if config.Source.Version.Major == 5 {
		config.Source.Tablespaces, err = greengage.TablespacesFromDB(db, utils.GetStateDirOldTablespacesFile())
		if err != nil {
			return Config{}, xerrors.Errorf("extract tablespace information: %w", err)
		}
	}

	return config, nil
}
