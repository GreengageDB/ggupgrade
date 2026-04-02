// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package greengage_test

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/blang/semver/v4"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/testutils"
)

func TestWaitForSegments(t *testing.T) {
	timeout := 30 * time.Second

	target := MustCreateCluster(t, greengage.SegConfigs{
		{DbID: 1, ContentID: -1, Hostname: "coordinator", DataDir: "/data/qddir/seg-1", Port: 15432, Role: greengage.PrimaryRole},
		{DbID: 2, ContentID: -1, Hostname: "standby", DataDir: "/data/standby", Port: 16432, Role: greengage.MirrorRole},
		{DbID: 3, ContentID: 0, Hostname: "sdw1", DataDir: "/data/dbfast1/seg1", Port: 25433, Role: greengage.PrimaryRole},
		{DbID: 4, ContentID: 0, Hostname: "sdw2", DataDir: "/data/dbfast_mirror1/seg1", Port: 25434, Role: greengage.MirrorRole},
		{DbID: 5, ContentID: 1, Hostname: "sdw2", DataDir: "/data/dbfast2/seg2", Port: 25435, Role: greengage.PrimaryRole},
		{DbID: 6, ContentID: 1, Hostname: "sdw1", DataDir: "/data/dbfast_mirror2/seg2", Port: 25436, Role: greengage.MirrorRole},
	})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("couldn't create sqlmock: %v", err)
	}
	defer testutils.FinishMock(mock, t)

	t.Run("succeeds", func(t *testing.T) {
		target.Version = semver.MustParse("6.0.0")

		expectFtsProbe(mock)
		expectGpSegmentConfigurationToReturn(mock, 4)
		expectPgStatReplicationToReturn(mock, 1, target.Version)

		err = greengage.WaitForSegments(db, timeout, target)
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}
	})

	t.Run("skips fts if GPDB version is 5", func(t *testing.T) {
		target.Version = semver.MustParse("5.0.0")

		expectGpSegmentConfigurationToReturn(mock, 4)
		expectPgStatReplicationToReturn(mock, 1, target.Version)

		err = greengage.WaitForSegments(db, timeout, target)
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}
	})

	t.Run("skips pg_stat_replication if there is no standby", func(t *testing.T) {
		target := MustCreateCluster(t, greengage.SegConfigs{
			{DbID: 1, ContentID: -1, Hostname: "coordinator", DataDir: "/data/qddir/seg-1", Port: 15432, Role: greengage.PrimaryRole},
			{DbID: 3, ContentID: 0, Hostname: "sdw1", DataDir: "/data/dbfast1/seg1", Port: 25433, Role: greengage.PrimaryRole},
			{DbID: 4, ContentID: 0, Hostname: "sdw2", DataDir: "/data/dbfast_mirror1/seg1", Port: 25434, Role: greengage.MirrorRole},
			{DbID: 5, ContentID: 1, Hostname: "sdw2", DataDir: "/data/dbfast2/seg2", Port: 25435, Role: greengage.PrimaryRole},
			{DbID: 6, ContentID: 1, Hostname: "sdw1", DataDir: "/data/dbfast_mirror2/seg2", Port: 25436, Role: greengage.MirrorRole},
		})
		target.Version = semver.MustParse("6.0.0")

		expectFtsProbe(mock)
		expectGpSegmentConfigurationToReturn(mock, 4)

		err = greengage.WaitForSegments(db, timeout, target)
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}
	})

	t.Run("does not check mode=s if there are no mirrors but has a standby", func(t *testing.T) {
		target := MustCreateCluster(t, greengage.SegConfigs{
			{DbID: 1, ContentID: -1, Hostname: "coordinator", DataDir: "/data/qddir/seg-1", Port: 15432, Role: greengage.PrimaryRole},
			{DbID: 2, ContentID: -1, Hostname: "standby", DataDir: "/data/standby", Port: 16432, Role: greengage.MirrorRole},
			{DbID: 3, ContentID: 0, Hostname: "sdw1", DataDir: "/data/dbfast1/seg1", Port: 25433, Role: greengage.PrimaryRole},
			{DbID: 5, ContentID: 1, Hostname: "sdw2", DataDir: "/data/dbfast2/seg2", Port: 25435, Role: greengage.PrimaryRole},
		})
		target.Version = semver.MustParse("6.0.0")

		expectFtsProbe(mock)
		expectGpSegmentConfigurationWithoutMirrorsToReturn(mock, 2)
		expectPgStatReplicationToReturn(mock, 1, target.Version)

		err = greengage.WaitForSegments(db, timeout, target)
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}
	})

	t.Run("skips mode=s and pg_stat_replication checks if there are no mirrors and no standby", func(t *testing.T) {
		target := MustCreateCluster(t, greengage.SegConfigs{
			{DbID: 1, ContentID: -1, Hostname: "coordinator", DataDir: "/data/qddir/seg-1", Port: 15432, Role: greengage.PrimaryRole},
			{DbID: 3, ContentID: 0, Hostname: "sdw1", DataDir: "/data/dbfast1/seg1", Port: 25433, Role: greengage.PrimaryRole},
			{DbID: 5, ContentID: 1, Hostname: "sdw2", DataDir: "/data/dbfast2/seg2", Port: 25435, Role: greengage.PrimaryRole},
		})
		target.Version = semver.MustParse("6.0.0")

		expectFtsProbe(mock)
		expectGpSegmentConfigurationWithoutMirrorsToReturn(mock, 2)

		err = greengage.WaitForSegments(db, timeout, target)
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}
	})

	t.Run("waits for segments to come up and standby to be synchronized", func(t *testing.T) {
		target.Version = semver.MustParse("6.0.0")

		expectFtsProbe(mock)
		expectGpSegmentConfigurationToReturn(mock, 0)
		expectFtsProbe(mock)
		expectGpSegmentConfigurationToReturn(mock, 4)
		expectPgStatReplicationToReturn(mock, 0, target.Version)
		expectFtsProbe(mock)
		expectGpSegmentConfigurationToReturn(mock, 4)
		expectPgStatReplicationToReturn(mock, 1, target.Version)

		err = greengage.WaitForSegments(db, timeout, target)
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}
	})

	t.Run("uses correct pg_stat_replication fields if GPDB version is 7", func(t *testing.T) {
		target.Version = semver.MustParse("7.0.0")

		expectFtsProbe(mock)
		expectGpSegmentConfigurationToReturn(mock, 0)
		expectFtsProbe(mock)
		expectGpSegmentConfigurationToReturn(mock, 4)
		expectPgStatReplicationToReturn(mock, 0, target.Version)
		expectFtsProbe(mock)
		expectGpSegmentConfigurationToReturn(mock, 4)
		expectPgStatReplicationToReturn(mock, 1, target.Version)

		err = greengage.WaitForSegments(db, timeout, target)
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}
	})

	t.Run("times out if segments never come up", func(t *testing.T) {
		target.Version = semver.MustParse("6.0.0")

		expectFtsProbe(mock)
		expectGpSegmentConfigurationToReturn(mock, 0)

		err = greengage.WaitForSegments(db, -1*time.Second, target)
		expected := "-1s timeout exceeded waiting for all segments to be up, in their preferred roles, and synchronized."
		if err.Error() != expected {
			t.Errorf("got: %#v want %s", err, expected)
		}
	})
}

func expectFtsProbe(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT gp_request_fts_probe_scan\(\);`).
		WillReturnRows(sqlmock.NewRows([]string{"gp_request_fts_probe_scan"}).AddRow("t"))
}

func expectGpSegmentConfigurationToReturn(mock sqlmock.Sqlmock, count int) {
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM gp_segment_configuration 
WHERE content > -1 AND status = 'u' AND \(role = preferred_role\) AND mode = 's'`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
}

func expectGpSegmentConfigurationWithoutMirrorsToReturn(mock sqlmock.Sqlmock, count int) {
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM gp_segment_configuration 
WHERE content > -1 AND status = 'u' AND \(role = preferred_role\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
}

func expectPgStatReplicationToReturn(mock sqlmock.Sqlmock, count int, version semver.Version) {
	whereClause := "sent_location = flush_location;"
	if version.Major > 6 {
		whereClause = "sent_lsn = flush_lsn;"
	}

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pg_stat_replication
WHERE state = 'streaming' AND ` + whereClause).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
}
