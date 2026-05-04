// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package greengage_test

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/blang/semver/v4"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/step"
	"github.com/GreengageDB/ggupgrade/testutils"
	"github.com/GreengageDB/ggupgrade/testutils/exectest"
	"github.com/GreengageDB/ggupgrade/testutils/testlog"
)

func TestHasMirrors(t *testing.T) {
	cases := []struct {
		name     string
		cluster  *greengage.Cluster
		expected bool
	}{
		{
			name: "returns true when cluster has mirrors and standby",
			cluster: MustCreateCluster(t, greengage.SegConfigs{
				{DbID: 1, ContentID: -1, Hostname: "coordinator", DataDir: "/data/qddir/seg-1", Port: 15432, Role: greengage.PrimaryRole},
				{DbID: 2, ContentID: -1, Hostname: "standby", DataDir: "/data/standby", Port: 16432, Role: greengage.MirrorRole},
				{DbID: 3, ContentID: 0, Hostname: "sdw1", DataDir: "/data/dbfast1/seg1", Port: 25433, Role: greengage.PrimaryRole},
				{DbID: 4, ContentID: 0, Hostname: "sdw2", DataDir: "/data/dbfast_mirror1/seg1", Port: 25434, Role: greengage.MirrorRole},
			}),
			expected: true,
		},
		{
			name: "returns false when cluster has no mirrors and standby",
			cluster: MustCreateCluster(t, greengage.SegConfigs{
				{DbID: 1, ContentID: -1, Hostname: "coordinator", DataDir: "/data/qddir/seg-1", Port: 15432, Role: greengage.PrimaryRole},
				{DbID: 2, ContentID: -1, Hostname: "standby", DataDir: "/data/standby", Port: 16432, Role: greengage.MirrorRole},
			}),
			expected: false,
		},
		{
			name: "returns false when cluster has no mirrors and no standby",
			cluster: MustCreateCluster(t, greengage.SegConfigs{
				{DbID: 1, ContentID: -1, Hostname: "coordinator", DataDir: "/data/qddir/seg-1", Port: 15432, Role: greengage.PrimaryRole},
			}),
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := c.cluster.HasMirrors()
			if actual != c.expected {
				t.Errorf("got %t want %t", actual, c.expected)
			}
		})
	}
}

func TestGetSegmentConfiguration(t *testing.T) {
	t.Run("can retrieve gp_segment_configuration", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("couldn't create sqlmock: %v", err)
		}
		defer testutils.FinishMock(mock, t)
		defer func() {
			if cErr := db.Close(); cErr != nil {
				t.Logf("error during Close: %+v", cErr)
			}
		}()

		rows := sqlmock.NewRows([]string{"dbid", "contentid", "port", "hostname", "address", "datadir", "role"})
		rows.AddRow(1, -1, 15432, "mdw", "mdw-1", "/data/qddir/seg-1", greengage.PrimaryRole)
		rows.AddRow(2, -1, 16432, "smdw", "smdw-1", "/data/standby", greengage.MirrorRole)
		rows.AddRow(3, 0, 25433, "sdw1", "sdw1-1", "/data/dbfast1/seg1", greengage.PrimaryRole)
		rows.AddRow(4, 0, 25434, "sdw2", "sdw2-2", "/data/dbfast_mirror1/seg1", greengage.MirrorRole)
		rows.AddRow(5, 1, 25435, "sdw2", "sdw2-2", "/data/dbfast2/seg2", greengage.PrimaryRole)
		rows.AddRow(6, 1, 25436, "sdw1", "sdw1-1", "/data/dbfast_mirror2/seg2", greengage.MirrorRole)

		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		actual, err := greengage.GetSegmentConfiguration(db, semver.Version{})
		if err != nil {
			t.Errorf("returned error %+v", err)
		}

		expected := greengage.SegConfigs{
			{DbID: 1, ContentID: -1, Port: 15432, Hostname: "mdw", Address: "mdw-1", DataDir: "/data/qddir/seg-1", Role: greengage.PrimaryRole},
			{DbID: 2, ContentID: -1, Port: 16432, Hostname: "smdw", Address: "smdw-1", DataDir: "/data/standby", Role: greengage.MirrorRole},
			{DbID: 3, ContentID: 0, Port: 25433, Hostname: "sdw1", Address: "sdw1-1", DataDir: "/data/dbfast1/seg1", Role: greengage.PrimaryRole},
			{DbID: 4, ContentID: 0, Port: 25434, Hostname: "sdw2", Address: "sdw2-2", DataDir: "/data/dbfast_mirror1/seg1", Role: greengage.MirrorRole},
			{DbID: 5, ContentID: 1, Port: 25435, Hostname: "sdw2", Address: "sdw2-2", DataDir: "/data/dbfast2/seg2", Role: greengage.PrimaryRole},
			{DbID: 6, ContentID: 1, Port: 25436, Hostname: "sdw1", Address: "sdw1-1", DataDir: "/data/dbfast_mirror2/seg2", Role: greengage.MirrorRole},
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("got configuration %+v, want %+v", actual, expected)
		}
	})

	t.Run("can retrieve gp_segment_configuration when all segements are on same host", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("couldn't create sqlmock: %v", err)
		}
		defer testutils.FinishMock(mock, t)
		defer func() {
			if cErr := db.Close(); cErr != nil {
				t.Logf("error during Close: %+v", cErr)
			}
		}()

		rows := sqlmock.NewRows([]string{"dbid", "contentid", "port", "hostname", "address", "datadir", "role"})
		rows.AddRow(1, -1, 15432, "mdw", "mdw-1", "/data/qddir/seg-1", greengage.PrimaryRole)
		rows.AddRow(2, -1, 16432, "mdw", "mdw-1", "/data/standby", greengage.MirrorRole)
		rows.AddRow(3, 0, 25433, "mdw", "mdw-1", "/data/dbfast1/seg1", greengage.PrimaryRole)
		rows.AddRow(4, 0, 25434, "mdw", "mdw-1", "/data/dbfast_mirror1/seg1", greengage.MirrorRole)
		rows.AddRow(5, 1, 25435, "mdw", "mdw-1", "/data/dbfast2/seg2", greengage.PrimaryRole)
		rows.AddRow(6, 1, 25436, "mdw", "mdw-1", "/data/dbfast_mirror2/seg2", greengage.MirrorRole)

		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		actual, err := greengage.GetSegmentConfiguration(db, semver.Version{})
		if err != nil {
			t.Errorf("returned error %+v", err)
		}

		expected := greengage.SegConfigs{
			{DbID: 1, ContentID: -1, Port: 15432, Hostname: "mdw", Address: "mdw-1", DataDir: "/data/qddir/seg-1", Role: greengage.PrimaryRole},
			{DbID: 2, ContentID: -1, Port: 16432, Hostname: "mdw", Address: "mdw-1", DataDir: "/data/standby", Role: greengage.MirrorRole},
			{DbID: 3, ContentID: 0, Port: 25433, Hostname: "mdw", Address: "mdw-1", DataDir: "/data/dbfast1/seg1", Role: greengage.PrimaryRole},
			{DbID: 4, ContentID: 0, Port: 25434, Hostname: "mdw", Address: "mdw-1", DataDir: "/data/dbfast_mirror1/seg1", Role: greengage.MirrorRole},
			{DbID: 5, ContentID: 1, Port: 25435, Hostname: "mdw", Address: "mdw-1", DataDir: "/data/dbfast2/seg2", Role: greengage.PrimaryRole},
			{DbID: 6, ContentID: 1, Port: 25436, Hostname: "mdw", Address: "mdw-1", DataDir: "/data/dbfast_mirror2/seg2", Role: greengage.MirrorRole},
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("got configuration %+v, want %+v", actual, expected)
		}
	})
}

func TestPrimaryHostnames(t *testing.T) {
	testStateDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Errorf("got error when creating tempdir: %+v", err)
	}
	expectedCluster := testutils.CreateMultinodeSampleCluster("/tmp")
	expectedCluster.GPHome = "/fake/path"
	expectedCluster.Version = semver.MustParse("6.0.0")
	testlog.SetupTestLogger()

	defer func() {
		os.RemoveAll(testStateDir)
	}()

	t.Run("returns a list of hosts for only the primaries", func(t *testing.T) {
		actual := expectedCluster.PrimaryHostnames()
		sort.Strings(actual)

		expected := []string{"host1", "host2"}
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("expected hostnames: %#v got: %#v", expected, actual)
		}
	})
}

func TestClusterFromDB(t *testing.T) {
	testStateDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Errorf("got error when creating tempdir: %+v", err)
	}

	testlog.SetupTestLogger()

	defer func() {
		os.RemoveAll(testStateDir)
	}()

	t.Run("returns an error if connection fails", func(t *testing.T) {
		greengage.SetVersionCommand(exectest.NewCommand(greengage.PostgresGPVersion_6_7_1))
		defer greengage.ResetVersionCommand()

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("couldn't create sqlmock: %v", err)
		}
		defer testutils.FinishMock(mock, t)

		expected := errors.New("connection failed")
		mock.ExpectQuery("SELECT ").WillReturnError(expected)

		actualCluster, err := greengage.ClusterFromDB(db, "", idl.ClusterDestination_source)
		if !errors.Is(err, expected) {
			t.Errorf("got %#v want %#v", err, expected)
		}

		if !reflect.DeepEqual(actualCluster, greengage.Cluster{}) {
			t.Errorf("got: %#v want empty cluster: %#v", actualCluster, &greengage.Cluster{})
		}
	})

	t.Run("returns an error if the segment configuration query fails", func(t *testing.T) {
		greengage.SetVersionCommand(exectest.NewCommand(greengage.PostgresGPVersion_6_7_1))
		defer greengage.ResetVersionCommand()

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("couldn't create sqlmock: %v", err)
		}
		defer testutils.FinishMock(mock, t)

		queryErr := errors.New("failed to get segment configuration")
		mock.ExpectQuery("SELECT .* FROM gp_segment_configuration").WillReturnError(queryErr)

		actualCluster, err := greengage.ClusterFromDB(db, "", idl.ClusterDestination_source)

		if err == nil {
			t.Errorf("Expected an error, but got nil")
		}
		if !reflect.DeepEqual(actualCluster, greengage.Cluster{}) {
			t.Errorf("Expected cluster to be empty, but got %#v", actualCluster)
		}
		if !strings.Contains(err.Error(), queryErr.Error()) {
			t.Errorf("Expected error: %+v got: %+v", queryErr.Error(), err.Error())
		}
	})

	t.Run("populates a cluster using DB information", func(t *testing.T) {
		greengage.SetVersionCommand(exectest.NewCommand(greengage.PostgresGPVersion_6_7_1))
		defer greengage.ResetVersionCommand()

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("couldn't create sqlmock: %v", err)
		}
		defer testutils.FinishMock(mock, t)

		mock.ExpectQuery("SELECT .* FROM gp_segment_configuration").WillReturnRows(testutils.MockSegmentConfiguration())

		gphome := "/usr/local/gpdb"
		version := semver.MustParse("6.7.1")
		destination := idl.ClusterDestination_intermediate
		actualCluster, err := greengage.ClusterFromDB(db, gphome, destination)
		if err != nil {
			t.Errorf("got unexpected error: %+v", err)
		}

		expectedCluster := testutils.MockCluster()
		expectedCluster.Destination = destination
		expectedCluster.Version = version
		expectedCluster.GPHome = gphome

		if !reflect.DeepEqual(&actualCluster, expectedCluster) {
			t.Errorf("got: %#v want: %#v ", &actualCluster, expectedCluster)
		}
	})
}

func TestSelectSegments(t *testing.T) {
	cluster := greengage.MustCreateCluster(t, greengage.SegConfigs{
		{ContentID: 1, Role: greengage.PrimaryRole},
		{ContentID: 2, Role: greengage.PrimaryRole},
		{ContentID: 3, Role: greengage.PrimaryRole},
		{ContentID: 3, Role: greengage.MirrorRole},
	})

	// Ensure all segments are visited correctly.
	actual := cluster.SelectSegments(func(cluster *greengage.SegConfig) bool {
		return true
	})
	sort.Sort(actual)

	expected := greengage.SegConfigs{
		{ContentID: 1, Role: greengage.PrimaryRole},
		{ContentID: 2, Role: greengage.PrimaryRole},
		{ContentID: 3, Role: greengage.PrimaryRole},
		{ContentID: 3, Role: greengage.MirrorRole},
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("SelectSegments(*) = %+v, want %+v", actual, expected)
	}

	// Test a simple selector.
	actual = cluster.SelectSegments(func(cluster *greengage.SegConfig) bool {
		return cluster.ContentID > 1
	})
	sort.Sort(actual)

	expected = greengage.SegConfigs{
		{ContentID: 2, Role: greengage.PrimaryRole},
		{ContentID: 3, Role: greengage.PrimaryRole},
		{ContentID: 3, Role: greengage.MirrorRole},
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("SelectSegments(ContentID > 1) = %+v, want %+v", actual, expected)
	}

}

func TestHasAllMirrorsAndStandby(t *testing.T) {
	t.Run("returns true on full cluster", func(t *testing.T) {
		segs := greengage.SegConfigs{
			{ContentID: -1, Role: greengage.PrimaryRole},
			{ContentID: -1, Role: greengage.MirrorRole},
			{ContentID: 0, Role: greengage.PrimaryRole},
			{ContentID: 0, Role: greengage.MirrorRole},
			{ContentID: 1, Role: greengage.PrimaryRole},
			{ContentID: 1, Role: greengage.MirrorRole},
			{ContentID: 2, Role: greengage.PrimaryRole},
			{ContentID: 2, Role: greengage.MirrorRole},
		}
		cluster := greengage.MustCreateCluster(t, segs)

		if !cluster.HasAllMirrorsAndStandby() {
			t.Errorf("expected a cluster that has all mirrors and a standby")
		}
	})

	cases := []struct {
		name string
		segs greengage.SegConfigs
	}{
		{
			"returns false on cluster with no mirrors",
			greengage.SegConfigs{
				{ContentID: -1, Role: greengage.PrimaryRole},
				{ContentID: 0, Role: greengage.PrimaryRole},
				{ContentID: 1, Role: greengage.PrimaryRole},
				{ContentID: 2, Role: greengage.PrimaryRole},
			},
		},
		{
			"returns false on cluster with mirrors but no standby",
			greengage.SegConfigs{
				{ContentID: -1, Role: greengage.PrimaryRole},
				{ContentID: 0, Role: greengage.PrimaryRole},
				{ContentID: 0, Role: greengage.MirrorRole},
				{ContentID: 1, Role: greengage.PrimaryRole},
				{ContentID: 1, Role: greengage.MirrorRole},
				{ContentID: 2, Role: greengage.PrimaryRole},
				{ContentID: 2, Role: greengage.MirrorRole},
			},
		},
		{
			"returns false on cluster with standby and no mirrors",
			greengage.SegConfigs{
				{ContentID: -1, Role: greengage.PrimaryRole},
				{ContentID: -1, Role: greengage.MirrorRole},
				{ContentID: 0, Role: greengage.PrimaryRole},
				{ContentID: 1, Role: greengage.PrimaryRole},
				{ContentID: 2, Role: greengage.PrimaryRole},
			},
		},
		{
			"returns false on cluster with only one mirror",
			greengage.SegConfigs{
				{ContentID: -1, Role: greengage.PrimaryRole},
				{ContentID: 0, Role: greengage.PrimaryRole},
				{ContentID: 0, Role: greengage.MirrorRole},
				{ContentID: 1, Role: greengage.PrimaryRole},
				{ContentID: 2, Role: greengage.PrimaryRole},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cluster := greengage.MustCreateCluster(t, c.segs)

			if cluster.HasAllMirrorsAndStandby() {
				t.Errorf("expected a cluster missing at least one mirror or its standby")
			}
		})
	}
}

func TestRunGreengageCmd(t *testing.T) {
	testlog.SetupTestLogger()

	cluster := MustCreateCluster(t, greengage.SegConfigs{
		{DbID: 1, ContentID: -1, Hostname: "coordinator", DataDir: "/data/qddir/seg-1", Port: 15432, Role: greengage.PrimaryRole},
	})
	cluster.GPHome = "/usr/local/greengage-db"

	t.Run("executes greengage utility with greengage_path.sh set and correct args", func(t *testing.T) {
		cmd := exectest.NewCommandWithVerifier(Success, func(name string, args ...string) {
			expected := "bash"
			if name != expected {
				t.Errorf("got %q want %q", name, expected)
			}

			expectedArgs := []string{"-c", "source /usr/local/greengage-db/greengage_path.sh && /usr/local/greengage-db/bin/gpaddmirrors -a -i mirrors_config --hba-hostnames"}
			if !reflect.DeepEqual(args, expectedArgs) {
				t.Errorf("got %q want %q", args, expectedArgs)
			}
		})
		greengage.SetGreengageCommand(cmd)
		defer greengage.ResetGreengageCommand()

		err := cluster.RunGreengageCmd(step.DevNullStream, "gpaddmirrors", "-a", "-i", "mirrors_config", "--hba-hostnames")
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}
	})

	t.Run("sets greengage environment variables", func(t *testing.T) {
		coordinatorDataDirectory := "MASTER_DATA_DIRECTORY"
		resetEnv := testutils.MustClearEnv(t, coordinatorDataDirectory)
		defer resetEnv()

		pgPort := "PGPORT"
		resetEnv = testutils.MustClearEnv(t, pgPort)
		defer resetEnv()

		// Echo the environment to stdout and to a copy for debugging
		greengage.SetGreengageCommand(exectest.NewCommand(EnvironmentMain))
		defer greengage.ResetGreengageCommand()

		streams := &step.BufferedStreams{}
		err := cluster.RunGreengageCmd(streams, "gpaddmirrors", "-a", "-i", "mirrors_config", "--hba-hostnames")
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}

		actual := streams.StdoutBuf.String()
		expected := "MASTER_DATA_DIRECTORY=/data/qddir/seg-1\nPGPORT=15432\n"
		if actual != expected {
			t.Errorf("got %q want %q", actual, expected)
		}
	})

	t.Run("returns errors", func(t *testing.T) {
		greengage.SetGreengageCommand(exectest.NewCommand(FailedMain))
		defer greengage.ResetGreengageCommand()

		err := cluster.RunGreengageCmd(step.DevNullStream, "gpaddmirrors", "-a", "-i", "mirrors_config", "--hba-hostnames")
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Errorf("got %T, want %T", err, exitError)
		}
	})
}

func TestGetCoordinatorSegPrefix(t *testing.T) {
	t.Run("returns a valid seg prefix given", func(t *testing.T) {
		cases := []struct {
			desc               string
			CoordinatorDataDir string
		}{
			{"an absolute path", "/data/coordinator/gpseg-1"},
			{"a relative path", "../coordinator/gpseg-1"},
			{"a implicitly relative path", "gpseg-1"},
		}

		for _, c := range cases {
			actual, err := greengage.GetCoordinatorSegPrefix(c.CoordinatorDataDir)
			if err != nil {
				t.Fatalf("got %#v, want nil", err)
			}

			expected := "gpseg"
			if actual != expected {
				t.Errorf("got %q, want %q", actual, expected)
			}
		}
	})

	t.Run("returns errors when given", func(t *testing.T) {
		cases := []struct {
			desc               string
			CoordinatorDataDir string
		}{
			{"the empty string", ""},
			{"a path without a content identifier", "/opt/myseg"},
			{"a path with a segment content identifier", "/opt/myseg2"},
			{"a path that is only a content identifier", "-1"},
			{"a path that ends in only a content identifier", "///-1"},
		}

		for _, c := range cases {
			_, err := greengage.GetCoordinatorSegPrefix(c.CoordinatorDataDir)
			if err == nil {
				t.Fatalf("got nil, want err")
			}
		}
	})
}

func MustCreateCluster(t *testing.T, segments greengage.SegConfigs) *greengage.Cluster {
	t.Helper()

	cluster, err := greengage.NewCluster(segments)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	return &cluster
}
