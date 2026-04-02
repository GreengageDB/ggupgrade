// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package greengage_test

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/blang/semver/v4"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/testutils"
	"github.com/GreengageDB/ggupgrade/utils"
)

func TestQueryPgStatActivity(t *testing.T) {
	target := MustCreateCluster(t, greengage.SegConfigs{
		{DbID: 1, ContentID: -1, Hostname: "coordinator", DataDir: "/data/qddir/seg-1", Port: 15432, Role: greengage.PrimaryRole},
		{DbID: 2, ContentID: -1, Hostname: "standby", DataDir: "/data/standby", Port: 16432, Role: greengage.MirrorRole},
		{DbID: 3, ContentID: 0, Hostname: "sdw1", DataDir: "/data/dbfast1/seg1", Port: 25433, Role: greengage.PrimaryRole},
		{DbID: 4, ContentID: 0, Hostname: "sdw2", DataDir: "/data/dbfast_mirror1/seg1", Port: 25434, Role: greengage.MirrorRole},
		{DbID: 5, ContentID: 1, Hostname: "sdw2", DataDir: "/data/dbfast2/seg2", Port: 25435, Role: greengage.PrimaryRole},
		{DbID: 6, ContentID: 1, Hostname: "sdw1", DataDir: "/data/dbfast_mirror2/seg2", Port: 25436, Role: greengage.MirrorRole},
	})
	target.Destination = idl.ClusterDestination_intermediate
	target.Version = semver.MustParse("6.0.0")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("couldn't create sqlmock: %v", err)
	}
	defer testutils.FinishMock(mock, t)

	t.Run("succeeds", func(t *testing.T) {
		expectPgStatActivityToNotReturn(mock)

		err = greengage.QueryPgStatActivity(db, target)
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}
	})

	t.Run("uses correct query for GPDB 5X", func(t *testing.T) {
		target.Version = semver.MustParse("5.0.0")
		defer func() {
			target.Version = semver.MustParse("6.0.0")
		}()

		mock.ExpectQuery(`SELECT application_name, usename, datname, current_query FROM pg_stat_activity WHERE procpid <> pg_backend_pid\(\) ORDER BY application_name, usename, datname;`).
			WillReturnRows(sqlmock.NewRows([]string{"application_name", "usename", "datname", "query"}))

		err = greengage.QueryPgStatActivity(db, target)
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}
	})

	t.Run("errors when pg_stat_activity shows active connections and database is NULL", func(t *testing.T) {
		expectPgStatActivityToReturn(mock).WillReturnRows(sqlmock.NewRows([]string{"application_name", "usename", "datname", "query"}).
			AddRow("etl_job", "gpadmin", nil, "SELECT * FROM my_table;").
			AddRow("status_checker", "gpcc", "stats_db", "SELECT * FROM stats;"))

		expected := greengage.StatActivities{
			{Application_name: sql.NullString{String: "etl_job"}, User: sql.NullString{String: "gpadmin"}, Datname: sql.NullString{String: "", Valid: false}, Query: sql.NullString{String: "SELECT * FROM my_table;"}},
			{Application_name: sql.NullString{String: "status_checker"}, User: sql.NullString{String: "gpcc"}, Datname: sql.NullString{String: "stats_db", Valid: true}, Query: sql.NullString{String: "SELECT * FROM stats;"}},
		}

		err = greengage.QueryPgStatActivity(db, target)
		var nextActionsErr utils.NextActionErr
		if !errors.As(err, &nextActionsErr) {
			t.Errorf("got type %T want %T", err, nextActionsErr)
		}

		if !strings.Contains(nextActionsErr.Err.Error(), expected.Error()) {
			t.Errorf("got %#v, want %#v", err, expected)
		}

		if !strings.Contains(nextActionsErr.NextAction, "close") {
			t.Errorf("got %q, want 'close'", nextActionsErr.NextAction)
		}
	})

	t.Run("errors when failing to query", func(t *testing.T) {
		expected := os.ErrPermission
		expectPgStatActivityToReturn(mock).WillReturnError(expected)

		err = greengage.QueryPgStatActivity(db, target)
		if !errors.Is(err, expected) {
			t.Errorf("got %v want %v", err, expected)
		}
	})

	t.Run("errors when failing to scan", func(t *testing.T) {
		expectPgStatActivityToReturn(mock).WillReturnRows(sqlmock.NewRows([]string{"application_name", "usename"}).
			AddRow("postgres", "gpadmin")) // return less fields than scan expects

		err = greengage.QueryPgStatActivity(db, target)
		if !strings.Contains(err.Error(), "Scan") {
			t.Errorf(`expected %v to contain "Scan"`, err)
		}
	})

	t.Run("errors when iterating the rows cals", func(t *testing.T) {
		expected := os.ErrPermission
		expectPgStatActivityToReturn(mock).WillReturnRows(sqlmock.NewRows([]string{"application_name", "usename", "datname", "query"}).
			AddRow("etl_job", "gpadmin", "postgres", "SELECT * FROM my_table;").
			RowError(0, expected))

		err = greengage.QueryPgStatActivity(db, target)
		if !errors.Is(err, expected) {
			t.Errorf("got %v want %v", err, expected)
		}
	})
}

func expectPgStatActivityToNotReturn(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT application_name, usename, datname, query FROM pg_stat_activity WHERE pid <> pg_backend_pid\(\) ORDER BY application_name, usename, datname;`).
		WillReturnRows(sqlmock.NewRows([]string{"application_name", "usename", "datname", "query"}))
}

func expectPgStatActivityToReturn(mock sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(`SELECT application_name, usename, datname, query FROM pg_stat_activity WHERE pid <> pg_backend_pid\(\) ORDER BY application_name, usename, datname;`)
}
