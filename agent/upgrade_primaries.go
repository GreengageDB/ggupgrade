// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
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

		targetMajorVersion := semver.MustParse(opt.TargetVersion).Major
		if targetMajorVersion <= 6 {
			err = RestoreTablespaces5Xto6X(opt.GetBackupDir(), opt.GetTablespaces(), opt.GetOldDBID(), opt.GetNewDataDir())
		} else {
			err = RestoreTablespacesTo7XAndNewer(opt.GetOldDBID(), opt.GetNewDataDir(), opt.TargetCatalogVersion, targetMajorVersion)
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
		if err := recreateSymlink(targetDir, symLinkName); err != nil {
			return xerrors.Errorf("recreate symbolic link: %w", err)
		}
	}

	return nil
}

func RestoreTablespacesTo7XAndNewer(oldDBID string, newDataDir string, targetCatalogVersion string, targetMajorVersion uint64) error {
	dbid, err := strconv.Atoi(oldDBID)
	if err != nil {
		return err
	}

	// Since the segment's data directories are a copy of the coordinator data directory, the
	// symbolic link in the pg_tblspc targets a tablespace directory for the coordinator.
	// We need to relink them to the appropriate tablespace directory and create a prefix directory
	// for the target version.
	//
	// Read tablespace links from pg_tblspc and link them to the tablespace directory with the
	//appropriate dbid. Then copy the target prefix from the coordinator. We need a non-empty prefix
	// since pg_upgrade does vacuum, and the absence of table files leads to the failure.

	targetTablespacePrefix := fmt.Sprintf("GPDB_%d_%s", targetMajorVersion, targetCatalogVersion)
	tablespaceDir := fmt.Sprintf("%s/pg_tblspc", newDataDir)
	tablespaces, err := utils.System.ReadDirFS(utils.System.DirFS(tablespaceDir), ".")
	if err != nil {
		return xerrors.Errorf("read tablespace directory: %w", err)
	}
	for _, spaceDir := range tablespaces {
		if spaceDir.Type() != fs.ModeSymlink {
			return xerrors.Errorf("The file pg_tblspc is not a symlink: %s", spaceDir.Name())
		}

		link, err := utils.System.ReadLink(filepath.Join(tablespaceDir, spaceDir.Name()))
		if err != nil {
			return xerrors.Errorf("read link: %w", err)
		}

		location := filepath.Dir(filepath.Clean(link))

		symLinkName := filepath.Join(tablespaceDir, spaceDir.Name())
		targetDir := filepath.Join(location, strconv.Itoa(dbid))

		coordinatorTableSpace := filepath.Join(link, targetTablespacePrefix)

		if err := recreateSymlink(targetDir, symLinkName); err != nil {
			return xerrors.Errorf("recreate symbolic link: %w", err)
		}
		options := []rsync.Option{
			rsync.WithSources(coordinatorTableSpace),
			rsync.WithDestination(symLinkName),
			rsync.WithOptions("--archive"),
		}

		if err := rsync.Rsync(options...); err != nil {
			return xerrors.Errorf("rsync coordinator tablespace directory to segment tablespace directory: %w", err)
		}
	}
	return nil
}

func recreateSymlink(sourceDir, symLinkName string) error {
	_, err := utils.System.Lstat(symLinkName)
	if err == nil {
		if err := utils.System.Remove(symLinkName); err != nil {
			return xerrors.Errorf("unlink %q: %w", symLinkName, err)
		}
	} else if !os.IsNotExist(err) {
		return xerrors.Errorf("stat symbolic link %q: %w", symLinkName, err)
	}

	if err := utils.System.Symlink(sourceDir, symLinkName); err != nil {
		return xerrors.Errorf("create symbolic link %q to directory %q: %w", symLinkName, sourceDir, err)
	}

	return nil
}
