// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package greengage_test

import (
	"reflect"
	"testing"

	"github.com/GreengageDB/ggupgrade/greengage"
)

func TestSelect(t *testing.T) {
	segs := greengage.SegConfigs{
		{ContentID: 1, Role: greengage.PrimaryRole},
		{ContentID: 2, Role: greengage.PrimaryRole},
		{ContentID: 3, Role: greengage.PrimaryRole},
		{ContentID: 3, Role: greengage.MirrorRole},
	}

	// Ensure all segments are visited correctly.
	selectAll := func(_ *greengage.SegConfig) bool { return true }
	results := segs.Select(selectAll)

	if !reflect.DeepEqual(results, segs) {
		t.Errorf("SelectSegments(*) = %+v, want %+v", results, segs)
	}

	// Test a simple selector.
	moreThanOne := func(c *greengage.SegConfig) bool { return c.ContentID > 1 }
	results = segs.Select(moreThanOne)

	expected := greengage.SegConfigs{segs[1], segs[2], segs[3]}
	if !reflect.DeepEqual(results, expected) {
		t.Errorf("SelectSegments(ContentID > 1) = %+v, want %+v", results, expected)
	}

}
