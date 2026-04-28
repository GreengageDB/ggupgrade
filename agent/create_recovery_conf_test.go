// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/GreengageDB/ggupgrade/agent"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/testutils"
	"github.com/GreengageDB/ggupgrade/testutils/testlog"
	"github.com/GreengageDB/ggupgrade/utils/errorlist"
)

func TestCreateRecoveryConf(t *testing.T) {
	testlog.SetupTestLogger()
	agentServer := agent.New()

	t.Run("creates recovery.conf for 5.x -> 6.x", func(t *testing.T) {
		mirrorDataDir := testutils.GetTempDir(t, "")
		defer testutils.MustRemoveAll(t, mirrorDataDir)

		connReqs := []*idl.CreateRecoveryConfRequest_Connection{{
			MirrorDataDir:      mirrorDataDir,
			User:               "gpadmin",
			PrimaryHost:        "sdw1",
			PrimaryPort:        int32(123),
			TargetMajorVersion: 6,
		}}

		_, err := agentServer.CreateRecoveryConf(context.Background(), &idl.CreateRecoveryConfRequest{Connections: connReqs})
		if err != nil {
			t.Errorf("unexpected error %#v", err)
		}

		contents := testutils.MustReadFile(t, filepath.Join(mirrorDataDir, "recovery.conf"))
		expected := `standby_mode = 'on'
primary_conninfo = 'user=gpadmin host=sdw1 port=123 sslmode=disable sslcompression=1 krbsrvname=postgres application_name=gp_walreceiver'
primary_slot_name = 'internal_wal_replication_slot'`

		if contents != expected {
			t.Errorf("got %q, want %q", contents, expected)
		}
	})

	t.Run("appends to postgresql.auto.conf and creates signal file for 6.x -> 7.x", func(t *testing.T) {
		mirrorDataDir := testutils.GetTempDir(t, "")
		defer testutils.MustRemoveAll(t, mirrorDataDir)

		testutils.MustWriteToFile(t, filepath.Join(mirrorDataDir, "postgresql.auto.conf"), "")

		connReqs := []*idl.CreateRecoveryConfRequest_Connection{{
			MirrorDataDir:      mirrorDataDir,
			User:               "gpadmin",
			PrimaryHost:        "sdw1",
			PrimaryPort:        int32(123),
			TargetMajorVersion: 7,
		}}

		_, err := agentServer.CreateRecoveryConf(context.Background(), &idl.CreateRecoveryConfRequest{Connections: connReqs})
		if err != nil {
			t.Errorf("unexpected error %#v", err)
		}

		contents := testutils.MustReadFile(t, filepath.Join(mirrorDataDir, "postgresql.auto.conf"))
		expected := `
primary_conninfo = 'user=gpadmin host=sdw1 port=123 sslmode=disable sslcompression=1 krbsrvname=postgres application_name=gp_walreceiver'
primary_slot_name = 'internal_wal_replication_slot'`

		if contents != expected {
			t.Errorf("got %q, want %q", contents, expected)
		}

		testutils.PathMustExist(t, filepath.Join(mirrorDataDir, "standby.signal"))
	})

	t.Run("returns multiple errors when failing to write recovery.conf", func(t *testing.T) {
		connReqs := []*idl.CreateRecoveryConfRequest_Connection{
			{
				MirrorDataDir:      "/does/not/exist",
				User:               "gpadmin",
				PrimaryHost:        "sdw1",
				PrimaryPort:        int32(123),
				TargetMajorVersion: 6,
			},
			{
				MirrorDataDir:      "/also/does/not/exist",
				User:               "gpadmin",
				PrimaryHost:        "sdw2",
				PrimaryPort:        int32(456),
				TargetMajorVersion: 6,
			}}

		_, err := agentServer.CreateRecoveryConf(context.Background(), &idl.CreateRecoveryConfRequest{Connections: connReqs})
		if err == nil {
			t.Error("expected error, returned nil")
		}

		var errs errorlist.Errors
		if !errors.As(err, &errs) {
			t.Fatalf("got error %#v, want type %T", err, errs)
		}

		if len(errs) != 2 {
			t.Errorf("got %d errors want 2", len(errs))
		}

		for _, err := range errs {
			var pathError *os.PathError
			if !errors.As(err, &pathError) {
				t.Errorf("got type %T want %T", err, pathError)
			}
		}
	})
}
