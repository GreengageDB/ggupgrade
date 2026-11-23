// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package hub_test

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/GreengageDB/ggupgrade/config/backupdir"
	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/hub"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/step"
	"github.com/GreengageDB/ggupgrade/testutils"
	"github.com/GreengageDB/ggupgrade/testutils/exectest"
	"github.com/GreengageDB/ggupgrade/testutils/testlog"
	"github.com/GreengageDB/ggupgrade/upgrade"
	"github.com/GreengageDB/ggupgrade/utils"
	"github.com/GreengageDB/ggupgrade/utils/errorlist"
	"github.com/GreengageDB/ggupgrade/utils/rsync"
	"github.com/blang/semver/v4"
	"golang.org/x/xerrors"
)

const FailureStdout = `
Checking for orphaned TOAST relations                       ok
Checking for gphdfs external tables                         ok
Checking for users assigned the gphdfs role                 fatal

| Your installation contains roles that have gphdfs privileges.
| These privileges need to be revoked before upgrade.  A list
| of roles and their corresponding gphdfs privileges that
| must be revoked is provided in the file:
|       gphdfs_user_roles.txt

Failure, exiting
`

const FailureWithTimingStdout = `
Checking for orphaned TOAST relations                       ok [ 1h36m ]
Checking for gphdfs external tables                         ok [ 12s ]
Checking for users assigned the gphdfs role                 fatal [ 36ms ]

| Your installation contains roles that have gphdfs privileges.
| These privileges need to be revoked before upgrade.  A list
| of roles and their corresponding gphdfs privileges that
| must be revoked is provided in the file:
|       gphdfs_user_roles.txt

Failure, exiting
`

func gpcommand() {
	args := os.Args
	fmt.Println(args)
	if strings.Contains(args[2], "gpstop") {
		os.Exit(1)
	}
}

func init() {

	exectest.RegisterMains(
		exectest.Success,
		exectest.Failure,
		gpcommand,
	)
}

func PgCheckFailure() {
	_, _ = os.Stdout.WriteString(FailureStdout)
	os.Exit(1)
}

func PgCheckFailureWithTiming() {
	_, _ = os.Stdout.WriteString(FailureWithTimingStdout)
	os.Exit(1)
}

// Writes to stdout and ignores any failure to do so.
func BlindlyWritingMain() {
	// Ignore SIGPIPE. Note that the obvious signal.Ignore(syscall.SIGPIPE)
	// doesn't work as expected; see https://github.com/golang/go/issues/32386.
	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)

	fmt.Println("blah blah blah blah")
	fmt.Println("blah blah blah blah")
	fmt.Println("blah blah blah blah")
}

func init() {
	exectest.RegisterMains(
		BlindlyWritingMain,
		PgCheckFailure,
		PgCheckFailureWithTiming,
	)
}

func TestUpgradeCoordinator(t *testing.T) {
	testlog.SetupTestLogger()

	now := time.Now()
	pgUpgradeTimestamp := now.Format(hub.TimeStringFormat)
	pgUpgradeDir, err := utils.GetPgUpgradeDir(greengage.PrimaryRole, -1, pgUpgradeTimestamp, "6.15.0")
	if err != nil {
		t.Fatal(err)
	}

	source := hub.MustCreateCluster(t, greengage.SegConfigs{
		{ContentID: -1, Port: 5432, DataDir: "/data/old", DbID: 1, Role: greengage.PrimaryRole},
		{ContentID: -1, Port: 5433, DataDir: "/data/standby", DbID: 2, Role: greengage.MirrorRole},
	})
	source.GPHome = "/usr/local/source"

	intermediate := hub.MustCreateCluster(t, greengage.SegConfigs{
		{ContentID: -1, Port: 5433, DataDir: "/data/new", DbID: 2, Role: greengage.PrimaryRole},
	})
	intermediate.GPHome = "/usr/local/target"
	intermediate.Version = semver.MustParse("6.15.0")

	backupDirs, err := backupdir.ParseParentBackupDirs("", *source)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("succeeds", func(t *testing.T) {
		upgrade.SetPgUpgradeCommand(exectest.NewCommand(hub.StreamingMain))
		defer upgrade.ResetPgUpgradeCommand()

		rsync.SetRsyncCommand(exectest.NewCommand(hub.Success))
		defer rsync.ResetRsyncCommand()

		streams := new(step.BufferedStreams)
		err := hub.UpgradeCoordinator(streams, backupDirs.CoordinatorBackupDir, false, false, 1, source, intermediate, idl.PgOptions_check, idl.Mode_copy, pgUpgradeTimestamp)
		if err != nil {
			t.Fatalf("unexpected error %+v", err)
		}

		stdout := streams.StdoutBuf.String()
		if stdout != hub.StreamingMainStdout {
			t.Errorf("got stdout %q, want %q", stdout, hub.StreamingMainStdout)
		}

		stderr := streams.StderrBuf.String()
		if stderr != hub.StreamingMainStderr {
			t.Errorf("got stderr %q, want %q", stderr, hub.StreamingMainStderr)
		}
	})

	t.Run("sets the old options when upgrading from GPDB 5 with a standby", func(t *testing.T) {
		upgrade.SetPgUpgradeCommand(exectest.NewCommandWithVerifier(hub.Success, func(command string, args ...string) {
			expected := "--old-options -x 2"
			if !strings.Contains(strings.Join(args, " "), expected) {
				t.Errorf("did not find %q in the args %q", expected, args)
			}
		}))
		defer upgrade.ResetPgUpgradeCommand()

		rsync.SetRsyncCommand(exectest.NewCommand(hub.Success))
		defer rsync.ResetRsyncCommand()

		source.Version = semver.MustParse("5.28.0")

		err := hub.UpgradeCoordinator(step.DevNullStream, backupDirs.CoordinatorBackupDir, false, false, 1, source, intermediate, idl.PgOptions_check, idl.Mode_copy, pgUpgradeTimestamp)
		if err != nil {
			t.Fatalf("unexpected error %+v", err)
		}
	})

	t.Run("does not set the old options when upgrading from GPDB 6 or later", func(t *testing.T) {
		upgrade.SetPgUpgradeCommand(exectest.NewCommandWithVerifier(hub.Success, func(command string, args ...string) {
			for _, arg := range args {
				if arg == "--old-options" {
					t.Errorf("expected --old-options to not be in args %q", args)
				}
			}
		}))
		defer upgrade.ResetPgUpgradeCommand()

		rsync.SetRsyncCommand(exectest.NewCommand(hub.Success))
		defer rsync.ResetRsyncCommand()

		source.Version = semver.MustParse("6.10.0")

		err := hub.UpgradeCoordinator(step.DevNullStream, backupDirs.CoordinatorBackupDir, false, false, 1, source, intermediate, idl.PgOptions_check, idl.Mode_copy, pgUpgradeTimestamp)
		if err != nil {
			t.Fatalf("unexpected error %+v", err)
		}
	})

	t.Run("restores the backup coordinator data directory", func(t *testing.T) {
		upgrade.SetPgUpgradeCommand(exectest.NewCommand(hub.Success))
		defer upgrade.ResetPgUpgradeCommand()

		var called bool
		rsync.SetRsyncCommand(exectest.NewCommandWithVerifier(hub.Success, func(utility string, args ...string) {
			called = true
		}))
		defer rsync.ResetRsyncCommand()

		err := hub.UpgradeCoordinator(step.DevNullStream, backupDirs.CoordinatorBackupDir, false, false, 1, source, intermediate, idl.PgOptions_check, idl.Mode_copy, pgUpgradeTimestamp)
		if err != nil {
			t.Fatalf("unexpected error %+v", err)
		}

		if !called {
			t.Error("expected rsync to be called")
		}
	})

	t.Run("errors when restoring the backup coordinator data directory fails", func(t *testing.T) {
		upgrade.SetPgUpgradeCommand(exectest.NewCommand(hub.Success))
		defer upgrade.ResetPgUpgradeCommand()

		rsync.SetRsyncCommand(exectest.NewCommand(hub.Failure))
		defer rsync.ResetRsyncCommand()

		err := hub.UpgradeCoordinator(step.DevNullStream, backupDirs.CoordinatorBackupDir, false, false, 1, source, intermediate, idl.PgOptions_upgrade, idl.Mode_copy, pgUpgradeTimestamp)
		var actual *exec.ExitError
		if !errors.As(err, &actual) {
			t.Fatalf("got %#v want ExitError", err)
		}

		if actual.ExitCode() != 1 {
			t.Errorf("got %d want 1", actual.ExitCode())
		}

		expected := fmt.Sprintf("rsync %q to %q: exit status 1", utils.GetCoordinatorPreUpgradeBackupDir(backupDirs.CoordinatorBackupDir)+string(os.PathSeparator), intermediate.CoordinatorDataDir())
		if err.Error() != expected {
			t.Errorf("got error message %q want %q", err.Error(), expected)
		}
	})

	t.Run("only when upgrading and not when running --check it restores the backup coordinator data directory", func(t *testing.T) {
		upgrade.SetPgUpgradeCommand(exectest.NewCommand(hub.Success))
		defer upgrade.ResetPgUpgradeCommand()

		rsync.SetRsyncCommand(exectest.NewCommandWithVerifier(hub.Success, func(utility string, args ...string) {
			if !strings.HasSuffix(utility, "rsync") {
				t.Errorf("got %q want rsync", utility)
			}

			options := args[:2]
			expectedOpts := []string{"--archive", "--delete"}
			if !reflect.DeepEqual(options, expectedOpts) {
				t.Errorf("got options %q want %q", options, expectedOpts)
			}

			src := args[2]
			expected := utils.GetCoordinatorPreUpgradeBackupDir(backupDirs.CoordinatorBackupDir) + string(os.PathSeparator)
			if src != expected {
				t.Errorf("got source %q want %q", src, expected)
			}

			dst := args[3]
			expected = intermediate.CoordinatorDataDir()
			if dst != expected {
				t.Errorf("got destination %q want %q", dst, expected)
			}

			exclusions := strings.Join(args[4:], " ")
			expected = "--exclude pg_log/*"
			if !reflect.DeepEqual(exclusions, expected) {
				t.Errorf("got exclusions %q want %q", exclusions, expected)
			}
		}))
		defer rsync.ResetRsyncCommand()

		err := hub.UpgradeCoordinator(step.DevNullStream, backupDirs.CoordinatorBackupDir, false, false, 1, source, intermediate, idl.PgOptions_upgrade, idl.Mode_copy, pgUpgradeTimestamp)
		if err != nil {
			t.Fatalf("unexpected error %+v", err)
		}
	})

	t.Run("errors when pg_upgrade fails and there is no error text", func(t *testing.T) {
		rsync.SetRsyncCommand(exectest.NewCommand(hub.Success))
		defer rsync.ResetRsyncCommand()

		upgrade.SetPgUpgradeCommand(exectest.NewCommand(hub.Failure))
		defer upgrade.ResetPgUpgradeCommand()

		err := hub.UpgradeCoordinator(new(step.BufferedStreams), backupDirs.CoordinatorBackupDir, false, false, 1, source, intermediate, idl.PgOptions_upgrade, idl.Mode_copy, pgUpgradeTimestamp)
		expected := "upgrade master: exit status 1"
		if err.Error() != expected {
			t.Errorf("got %q want %q", err.Error(), expected)
		}
	})

	t.Run("returns next actions error when pg_upgrade check fails with context", func(t *testing.T) {
		utils.System.Now = func() time.Time {
			return now
		}
		utils.System.MkdirAll = func(path string, perms os.FileMode) error {
			if path != pgUpgradeDir {
				t.Fatalf("got pg_upgrade working directory %q want %q", path, pgUpgradeDir)
			}

			testutils.MustRemoveAll(t, pgUpgradeDir)
			err := os.MkdirAll(path, perms)
			if err != nil {
				return err
			}

			testutils.MustWriteToFile(t, filepath.Join(pgUpgradeDir, "gphdfs_user_roles.txt"), "")
			return nil
		}
		defer utils.ResetSystemFunctions()

		rsync.SetRsyncCommand(exectest.NewCommand(hub.Success))
		defer rsync.ResetRsyncCommand()

		upgrade.SetPgUpgradeCommand(exectest.NewCommand(PgCheckFailure))
		defer upgrade.ResetPgUpgradeCommand()

		err := hub.UpgradeCoordinator(new(step.BufferedStreams), backupDirs.CoordinatorBackupDir, false, false, 1, source, intermediate, idl.PgOptions_check, idl.Mode_copy, pgUpgradeTimestamp)
		var nextActionsErr utils.NextActionErr
		if !errors.As(err, &nextActionsErr) {
			t.Fatalf("got type %T want %T", err, nextActionsErr)
		}

		nextAction := "Consult the pg_upgrade check output files located"
		if !strings.Contains(nextActionsErr.NextAction, nextAction) {
			t.Errorf("got %q want %q", nextActionsErr.NextAction, nextAction)
		}

		nextAction = `If you haven't already run the "initialize" data migration scripts with`
		if !strings.Contains(nextActionsErr.NextAction, nextAction) {
			t.Errorf("got %q want %q", nextActionsErr.NextAction, nextAction)
		}

		expected := errors.New("check master: exit status 1")
		if errors.Is(err, expected) {
			t.Errorf("got %v want %v", err, expected)
		}
	})

	t.Run("returns an error if the command succeeds but the io.Writer fails", func(t *testing.T) {
		rsync.SetRsyncCommand(exectest.NewCommand(hub.Success))
		defer rsync.ResetRsyncCommand()

		// Don't fail in the subprocess even when the stdout stream is closed.
		upgrade.SetPgUpgradeCommand(exectest.NewCommand(BlindlyWritingMain))
		defer upgrade.ResetPgUpgradeCommand()

		err := hub.UpgradeCoordinator(testutils.FailingStreams{Err: errors.New("write failed")}, backupDirs.CoordinatorBackupDir, false, false, 1, source, intermediate, idl.PgOptions_upgrade, idl.Mode_copy, pgUpgradeTimestamp)
		expected := "upgrade master: write failed"
		if err.Error() != expected {
			t.Errorf("got %q want %q", err.Error(), expected)
		}
	})
}

func TestRsyncCoordinatorDir(t *testing.T) {
	t.Run("rsync streams stdout and stderr to the client", func(t *testing.T) {
		rsync.SetRsyncCommand(exectest.NewCommand(hub.StreamingMain))
		defer rsync.ResetRsyncCommand()

		stream := new(step.BufferedStreams)
		err := hub.RsyncCoordinatorDataDir(stream, "", "")

		if err != nil {
			t.Errorf("returned: %+v", err)
		}

		stdout := stream.StdoutBuf.String()
		if stdout != hub.StreamingMainStdout {
			t.Errorf("got stdout %q, want %q", stdout, hub.StreamingMainStdout)
		}

		stderr := stream.StderrBuf.String()
		if stderr != hub.StreamingMainStderr {
			t.Errorf("got stderr %q, want %q", stderr, hub.StreamingMainStderr)
		}
	})
}

func TestUpdateIntermediateTablespaceOids(t *testing.T) {
	query := `select spcname, oid from pg_tablespace where spcname not in \('pg_default', 'pg_global'\);`

	source := hub.MustCreateCluster(t, greengage.SegConfigs{
		{ContentID: -1, Port: 5432, DataDir: "/data/cdw", DbID: 1, Role: greengage.PrimaryRole},
		{ContentID: -1, Port: 5433, DataDir: "/data/standby", DbID: 2, Role: greengage.MirrorRole},
		{ContentID: 0, Port: 6000, DataDir: "/data/sdw1", DbID: 3, Role: greengage.PrimaryRole},
		{ContentID: 0, Port: 6000, DataDir: "/data/mdw1", DbID: 4, Role: greengage.MirrorRole},
		{ContentID: 1, Port: 6001, DataDir: "/data/sdw2", DbID: 5, Role: greengage.PrimaryRole},
		{ContentID: 2, Port: 6001, DataDir: "/data/mdw2", DbID: 6, Role: greengage.MirrorRole},
	})
	intermediate := hub.MustCreateCluster(t, greengage.SegConfigs{
		{ContentID: -1, Port: 5432, DataDir: "/tmp/", DbID: 1, Role: greengage.PrimaryRole},
	})

	tablespaces := greengage.Tablespaces{
		-1: {
			16000: {
				Location:    "/tmp/ts1",
				Name:        "ts1",
				UserDefined: true,
			},
			17000: {
				Location:    "/tmp/ts2",
				Name:        "ts2",
				UserDefined: true,
			},
		},
		0: {
			16000: {
				Location:    "/tmp/ts1",
				Name:        "ts1",
				UserDefined: true,
			},
			17000: {
				Location:    "/tmp/ts2",
				Name:        "ts2",
				UserDefined: true,
			},
		},
		1: {
			16000: {
				Location:    "/tmp/ts3",
				Name:        "ts1",
				UserDefined: true,
			},
			17000: {
				Location:    "/tmp/ts2",
				Name:        "ts2",
				UserDefined: true,
			},
		},
	}

	t.Run("UpdateIntermediateTablespaceOids success", func(t *testing.T) {
		greengage.SetGreengageCommand(exectest.NewCommand(hub.Success))
		defer greengage.ResetGreengageCommand()

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer testutils.FinishMock(mock, t)

		rows := mock.NewRows([]string{"spcname", "oid"})
		rows.AddRow("ts1", 26000)
		rows.AddRow("ts2", 27000)
		mock.ExpectQuery(query).WillReturnRows(rows)
		mock.ExpectClose()

		openDB := func(driver, dsn string) (*sql.DB, error) {
			return db, nil
		}

		source.Tablespaces = tablespaces
		defer func() {
			source.Tablespaces = nil
		}()
		err = hub.UpdateIntermediateTablespaceOids(source, intermediate, openDB)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		tablespaces := greengage.Tablespaces{
			-1: {
				26000: {
					Location:    "/tmp/ts1",
					Name:        "ts1",
					UserDefined: true,
				},
				27000: {
					Location:    "/tmp/ts2",
					Name:        "ts2",
					UserDefined: true,
				},
			},
			0: {
				26000: {
					Location:    "/tmp/ts1",
					Name:        "ts1",
					UserDefined: true,
				},
				27000: {
					Location:    "/tmp/ts2",
					Name:        "ts2",
					UserDefined: true,
				},
			},
			1: {
				26000: {
					Location:    "/tmp/ts3",
					Name:        "ts1",
					UserDefined: true,
				},
				27000: {
					Location:    "/tmp/ts2",
					Name:        "ts2",
					UserDefined: true,
				},
			},
		}
		if !reflect.DeepEqual(intermediate.Tablespaces, tablespaces) {
			t.Errorf("got tablespaces %#v, want %#v", intermediate.Tablespaces, tablespaces)
		}
	})

	t.Run("Fail to start intermediate cluster", func(t *testing.T) {
		greengage.SetGreengageCommand(exectest.NewCommand(hub.Failure))
		defer greengage.ResetGreengageCommand()
		openDB := func(driver, dsn string) (*sql.DB, error) {
			return nil, nil
		}

		var exitErr *exec.ExitError
		err := hub.UpdateIntermediateTablespaceOids(source, intermediate, openDB)
		if !errors.As(err, &exitErr) {
			t.Errorf("got %v, want %v", err, os.ErrPermission)
		}
	})

	t.Run("opendb fails", func(t *testing.T) {
		greengage.SetGreengageCommand(exectest.NewCommand(hub.Success))
		defer greengage.ResetGreengageCommand()
		expected := errors.New("opendb fails")
		openDB := func(driver, dsn string) (*sql.DB, error) {
			return nil, expected
		}

		err := hub.UpdateIntermediateTablespaceOids(source, intermediate, openDB)
		if !errors.Is(err, expected) {
			t.Errorf("got %v, want %v", err, expected)
		}
	})

	t.Run("MapSpcname2Oid errors", func(t *testing.T) {
		greengage.SetGreengageCommand(exectest.NewCommand(hub.Success))
		defer greengage.ResetGreengageCommand()
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer testutils.FinishMock(mock, t)

		expected := errors.New("MapSpcname2Oid fails")
		mock.ExpectQuery(query).WillReturnError(expected)
		mock.ExpectClose()

		openDB := func(driver, dsn string) (*sql.DB, error) {
			return db, nil
		}
		err = hub.UpdateIntermediateTablespaceOids(source, intermediate, openDB)
		if !errors.Is(err, expected) {
			t.Errorf("got %v, want %v", err, expected)
		}
	})

	t.Run("intermediate cluster lack of the tablespace", func(t *testing.T) {
		greengage.SetGreengageCommand(exectest.NewCommand(hub.Success))
		defer greengage.ResetGreengageCommand()

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer testutils.FinishMock(mock, t)

		rows := mock.NewRows([]string{"spcname", "oid"})
		rows.AddRow("ts1", 26000)
		mock.ExpectQuery(query).WillReturnRows(rows)
		mock.ExpectClose()

		openDB := func(driver, dsn string) (*sql.DB, error) {
			return db, nil
		}

		source.Tablespaces = tablespaces
		defer func() {
			source.Tablespaces = nil
		}()

		expected := xerrors.Errorf("the intermediate cluster does not have tablespace %s, while the source cluster has it.", "ts2")

		err = hub.UpdateIntermediateTablespaceOids(source, intermediate, openDB)
		if err.Error() != expected.Error() {
			t.Errorf("got %v, want %v", err, expected)
		}
	})

	t.Run("Stop intermediate cluster error", func(t *testing.T) {
		greengage.SetIsCoordinatorRunningCommand(exectest.NewCommand(exectest.Success))
		defer greengage.ResetIsCoordinatorRunningCommand()

		fd, err := os.Create("/tmp/postmaster.pid")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove("/tmp/postmaster.pid")
		err = fd.Close()
		if err != nil {
			t.Fatal(err)
		}

		greengage.SetGreengageCommand(exectest.NewCommand(gpcommand))
		defer greengage.ResetGreengageCommand()

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer testutils.FinishMock(mock, t)

		rows := mock.NewRows([]string{"spcname", "oid"})
		rows.AddRow("ts1", 26000)
		rows.AddRow("ts2", 27000)
		mock.ExpectQuery(query).WillReturnRows(rows)
		mock.ExpectClose()

		openDB := func(driver, dsn string) (*sql.DB, error) {
			return db, nil
		}

		source.Tablespaces = tablespaces
		defer func() {
			source.Tablespaces = nil
		}()

		err = hub.UpdateIntermediateTablespaceOids(source, intermediate, openDB)

		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Errorf("got %v, want %v", err, exitErr)
		}
	})

	t.Run("multiply errors", func(t *testing.T) {
		greengage.SetIsCoordinatorRunningCommand(exectest.NewCommand(exectest.Success))
		defer greengage.ResetIsCoordinatorRunningCommand()

		fd, err := os.Create("/tmp/postmaster.pid")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove("/tmp/postmaster.pid")
		err = fd.Close()
		if err != nil {
			t.Fatal(err)
		}

		greengage.SetGreengageCommand(exectest.NewCommand(gpcommand))
		defer greengage.ResetGreengageCommand()

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer testutils.FinishMock(mock, t)

		rows := mock.NewRows([]string{"spcname", "oid"})
		rows.AddRow("ts1", 26000)
		mock.ExpectQuery(query).WillReturnRows(rows)

		expectedCloseError := errors.New("close error")

		mock.ExpectClose().WillReturnError(expectedCloseError)

		openDB := func(driver, dsn string) (*sql.DB, error) {
			return db, nil
		}

		source.Tablespaces = tablespaces
		defer func() {
			source.Tablespaces = nil
		}()

		err = hub.UpdateIntermediateTablespaceOids(source, intermediate, openDB)

		var errList errorlist.Errors
		if !errors.As(err, &errList) {
			t.Errorf("got %v, want %v", err, errList)
		}

		if len(errList) != 3 {
			t.Errorf("got %v, want %v", len(errList), 3)
		}

		expectedMissError := xerrors.Errorf("the intermediate cluster does not have tablespace %s, while the source cluster has it.", "ts2")

		if errList[0].Error() != expectedMissError.Error() {
			t.Errorf("got %v, want %v", errList[0], expectedMissError)
		}
		if !errors.Is(errList[1], expectedCloseError) {
			t.Errorf("got %v, want %v", errList[1], expectedCloseError)
		}
		var exitErr *exec.ExitError
		if !errors.As(errList[2], &exitErr) {
			t.Errorf("got %v, want %v", err, exitErr)
		}
	})
}
