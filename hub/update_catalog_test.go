// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package hub_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/hub"
	"github.com/GreengageDB/ggupgrade/testutils"
	"github.com/GreengageDB/ggupgrade/utils/errorlist"
)

func TestUpdateGpSegmentConfiguration(t *testing.T) {
	// success cases
	cases := []struct {
		name   string
		target *greengage.Cluster
	}{
		{
			name: "updates ports for every segment",
			target: hub.MustCreateCluster(t, greengage.SegConfigs{
				{DbID: 1, ContentID: -1, Port: 123, Role: greengage.PrimaryRole},
				{DbID: 8, ContentID: -1, Port: 789, Role: greengage.MirrorRole},
				{DbID: 2, ContentID: 0, Port: 234, Role: greengage.PrimaryRole},
				{DbID: 5, ContentID: 0, Port: 111, Role: greengage.MirrorRole},
				{DbID: 3, ContentID: 1, Port: 345, Role: greengage.PrimaryRole},
				{DbID: 6, ContentID: 1, Port: 222, Role: greengage.MirrorRole},
				{DbID: 4, ContentID: 2, Port: 456, Role: greengage.PrimaryRole},
				{DbID: 7, ContentID: 2, Port: 333, Role: greengage.MirrorRole},
			}),
		},
		{
			name: "updates ports when there is no standby or mirrors",
			target: hub.MustCreateCluster(t, greengage.SegConfigs{
				{DbID: 1, ContentID: -1, Port: 123, Role: greengage.PrimaryRole},
				{DbID: 2, ContentID: 0, Port: 234, Role: greengage.PrimaryRole},
				{DbID: 3, ContentID: 1, Port: 345, Role: greengage.PrimaryRole},
				{DbID: 4, ContentID: 2, Port: 456, Role: greengage.PrimaryRole},
			}),
		},
		{
			name: "updates ports when there is a standby but no mirrors",
			target: hub.MustCreateCluster(t, greengage.SegConfigs{
				{DbID: 1, ContentID: -1, Port: 123, Role: greengage.PrimaryRole},
				{DbID: 5, ContentID: -1, Port: 789, Role: greengage.MirrorRole},
				{DbID: 2, ContentID: 0, Port: 234, Role: greengage.PrimaryRole},
				{DbID: 3, ContentID: 1, Port: 345, Role: greengage.PrimaryRole},
				{DbID: 4, ContentID: 2, Port: 456, Role: greengage.PrimaryRole},
			}),
		},
		{
			name: "updates ports when there is no standby but mirrors",
			target: hub.MustCreateCluster(t, greengage.SegConfigs{
				{DbID: 1, ContentID: -1, Port: 123, Role: greengage.PrimaryRole},
				{DbID: 2, ContentID: 0, Port: 234, Role: greengage.PrimaryRole},
				{DbID: 5, ContentID: 0, Port: 111, Role: greengage.MirrorRole},
				{DbID: 3, ContentID: 1, Port: 345, Role: greengage.PrimaryRole},
				{DbID: 6, ContentID: 1, Port: 222, Role: greengage.MirrorRole},
				{DbID: 4, ContentID: 2, Port: 456, Role: greengage.PrimaryRole},
				{DbID: 7, ContentID: 2, Port: 333, Role: greengage.MirrorRole},
			}),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock: %v", err)
			}
			defer testutils.FinishMock(mock, t)
			defer func() {
				if cErr := db.Close(); cErr != nil {
					t.Logf("error during Close: %+v", cErr)
				}
			}()

			mock.MatchExpectationsInOrder(false) // since we iterate over maps for which golang does not guarantee order

			mock.ExpectBegin()

			for _, primary := range c.target.Primaries {
				expectCatalogUpdate(mock, primary).WillReturnResult(sqlmock.NewResult(0, 1))
			}

			for _, mirror := range c.target.Mirrors {
				expectCatalogUpdate(mock, mirror).WillReturnResult(sqlmock.NewResult(0, 1))
			}

			mock.ExpectCommit()

			err = hub.UpdateGpSegmentConfiguration(db, c.target)
			if err != nil {
				t.Errorf("returned error %+v", err)
			}
		})
	}

	// error cases
	target := hub.MustCreateCluster(t, greengage.SegConfigs{
		{DbID: 1, ContentID: -1, Port: 123, Role: greengage.PrimaryRole},
		{DbID: 8, ContentID: -1, Port: 789, Role: greengage.MirrorRole},
		{DbID: 2, ContentID: 0, Port: 234, Role: greengage.PrimaryRole},
		{DbID: 5, ContentID: 0, Port: 111, Role: greengage.MirrorRole},
		{DbID: 3, ContentID: 1, Port: 345, Role: greengage.PrimaryRole},
		{DbID: 6, ContentID: 1, Port: 222, Role: greengage.MirrorRole},
		{DbID: 4, ContentID: 2, Port: 456, Role: greengage.PrimaryRole},
		{DbID: 7, ContentID: 2, Port: 333, Role: greengage.MirrorRole},
	})

	expected := fmt.Errorf("sentinel error")
	ErrRollback := fmt.Errorf("rollback failed")

	errorCases := []struct {
		name          string
		expectations  func(sqlmock.Sqlmock)
		verifications func(*testing.T, error)
	}{{
		name: "errors when beginning a transaction fails",
		expectations: func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin().WillReturnError(expected)
		},
		verifications: func(t *testing.T, actual error) {
			if !errors.Is(actual, expected) {
				t.Errorf("got %#v want %#v", actual, expected)
			}
		},
	}, {
		name: "rolls back transaction when updating the catalog fails",
		expectations: func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE gp_segment_configuration SET").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE gp_segment_configuration SET").WillReturnError(expected)
			mock.ExpectRollback()
		},
		verifications: func(t *testing.T, actual error) {
			if !errors.Is(actual, expected) {
				t.Errorf("got %#v want %#v", actual, expected)
			}
		},
	}, {
		name: "errors when commit fails",
		expectations: func(mock sqlmock.Sqlmock) {
			mock.MatchExpectationsInOrder(false) // since we iterate over maps for which golang does not guarantee order
			mock.ExpectBegin()
			for _, primary := range target.Primaries {
				expectCatalogUpdate(mock, primary).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			for _, mirror := range target.Mirrors {
				expectCatalogUpdate(mock, mirror).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectCommit().WillReturnError(expected)
		},
		verifications: func(t *testing.T, actual error) {
			if !errors.Is(actual, expected) {
				t.Errorf("got %#v want %#v", actual, expected)
			}
		},
	}, {
		name: "errors when rolling back fails",
		expectations: func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE gp_segment_configuration SET").WillReturnError(expected)
			mock.ExpectRollback().WillReturnError(ErrRollback)
		},
		verifications: func(t *testing.T, err error) {
			var errs errorlist.Errors
			if !errors.As(err, &errs) {
				t.Fatalf("error %#v does not contain type %T", err, errs)
			}
			if !errors.Is(errs[0], expected) {
				t.Errorf("got %#v want %#v", err, expected)
			}
			if !errors.Is(errs[1], ErrRollback) {
				t.Errorf("got %#v want %#v", err, ErrRollback)
			}
		},
	}, {
		name: "rolls back if updating gp_segment_configuration returns multiple rows",
		expectations: func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE gp_segment_configuration SET").WillReturnResult(sqlmock.NewResult(0, 2))
			mock.ExpectRollback()
		},
		verifications: func(t *testing.T, err error) {
			expected := "Expected 1 row to be updated for segment dbid"
			if !strings.HasPrefix(err.Error(), expected) {
				t.Errorf("got %q want %q", err.Error(), expected)
			}
		},
	}}

	for _, c := range errorCases {
		t.Run(c.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock: %v", err)
			}
			defer testutils.FinishMock(mock, t)
			defer func() {
				if cErr := db.Close(); cErr != nil {
					t.Logf("error during Close: %+v", cErr)
				}
			}()

			c.expectations(mock)
			err = hub.UpdateGpSegmentConfiguration(db, target)
			c.verifications(t, err)
		})
	}
}

func expectCatalogUpdate(mock sqlmock.Sqlmock, seg greengage.SegConfig) *sqlmock.ExpectedExec {
	return mock.ExpectExec("UPDATE gp_segment_configuration SET port = (.+), datadir = (.+) WHERE content = (.+) AND role = (.+)").
		WithArgs(seg.Port, seg.DataDir, seg.ContentID, seg.Role)
}
