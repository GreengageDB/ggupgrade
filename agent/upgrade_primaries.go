// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/blang/semver/v4"
	"golang.org/x/xerrors"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/upgrade"
	"github.com/GreengageDB/ggupgrade/utils"
	"github.com/GreengageDB/ggupgrade/utils/errorlist"
	"github.com/GreengageDB/ggupgrade/utils/rsync"
)

func (s *Server) UpgradePrimaries(ctx context.Context, req *idl.UpgradePrimariesRequest) (*idl.UpgradePrimariesReply, error) {
	log.Printf("starting %s", req.GetAction())

	err := upgradePrimariesInParallel(req.GetOpts())
	if err != nil {
		return &idl.UpgradePrimariesReply{}, err
	}

	return &idl.UpgradePrimariesReply{}, nil
}

func upgradePrimariesInParallel(opts []*idl.PgOptions) error {
	host, err := utils.System.Hostname()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(opts))

	for _, opt := range opts {
		wg.Add(1)
		go func(host string, opt *idl.PgOptions) {
			defer wg.Done()

			errs <- upgradePrimarySegment(host, opt)
		}(host, opt)
	}

	wg.Wait()
	close(errs)

	for e := range errs {
		err = errorlist.Append(err, e)
	}

	return err
}

func upgradePrimarySegment(host string, opt *idl.PgOptions) error {
	if opt.GetAction() != idl.PgOptions_check {
		err := restoreBackup(opt.GetBackupDir(), opt.GetNewDataDir())
		if err != nil {
			return xerrors.Errorf("restore backup of upgraded master data directory on host %s for content id %d: %w", host, opt.GetContentID(), err)
		}

		targetMajorVersion, err := semver.Parse(opt.TargetVersion)
		if err != nil {
			return xerrors.Errorf("parse target version: %w", err)
		}

		if targetMajorVersion.Major <= 6 {
			err = RestoreTablespaces5Xto6X(opt.GetBackupDir(), opt.GetTablespaces(), opt.GetOldDBID(), opt.GetNewDataDir())
		} else {
			err = upgrade.RelinkTablespaces(opt.GetOldDBID(), opt.GetNewDataDir(), opt.GetTablespaces())
		}

		if err != nil {
			return xerrors.Errorf("restore tablespace on host %s for content id %d: %w", host, opt.GetContentID(), err)
		}
	}

	err := upgrade.Run(io.Discard, io.Discard, opt)
	if err != nil {
		return xerrors.Errorf("%s primary on host %s with content %d: %w", opt.GetAction(), host, opt.GetContentID(), err)
	}

	return nil
}

func restoreBackup(backupDir string, newDataDir string) error {
	options := []rsync.Option{
		rsync.WithSources(utils.GetCoordinatorPostUpgradeBackupDir(backupDir) + string(os.PathSeparator)),
		rsync.WithDestination(newDataDir),
		rsync.WithOptions("--archive", "--delete"),
		rsync.WithExcludedFiles(
			"internal.auto.conf",
			"postgresql.conf",
			"pg_hba.conf",
			"postmaster.opts",
			"gp_dbid",
			"gpssh.conf",
			"gpperfmon"),
	}

	return rsync.Rsync(options...)
}

func RestoreTablespaces5Xto6X(backupDir string, tablespaces map[int32]*idl.TablespaceInfo, oldDBID string, newDataDir string) error {
	dbid, err := strconv.Atoi(oldDBID)
	if err != nil {
		return err
	}

	for oid, tablespace := range tablespaces {
		if !tablespace.GetUserDefined() {
			continue
		}

		targetDir := greengage.GetTablespaceLocationForDbId(tablespace, dbid)
		sourceDir := greengage.GetCoordinatorTablespaceLocation(utils.GetTablespaceBackupDir(backupDir), int(oid)) + string(os.PathSeparator)

		options := []rsync.Option{
			rsync.WithSources(sourceDir),
			rsync.WithDestination(targetDir),
			rsync.WithOptions("--archive", "--delete"),
		}

		if err := rsync.Rsync(options...); err != nil {
			return xerrors.Errorf("rsync master tablespace directory to segment tablespace directory: %w", err)
		}

		symLinkName := fmt.Sprintf("%s/pg_tblspc/%s", newDataDir, strconv.Itoa(int(oid)))
		if err := utils.RecreateSymlink(targetDir, symLinkName); err != nil {
			return xerrors.Errorf("recreate symbolic link: %w", err)
		}
	}

	return nil
}
