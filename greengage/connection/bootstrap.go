// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"database/sql"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/idl"
)

// Bootstrap returns a sql.DB connection. Most callers will use the Connection
// function on the cluster object. However, Bootstrap is useful for when a
// cluster object does not exist and a database connection is needed.
func Bootstrap(destination idl.ClusterDestination, gphome string, port int, database string) (*sql.DB, error) {
	cluster, err := greengage.NewCluster([]greengage.SegConfig{})
	if err != nil {
		return nil, err
	}

	// destination and version are needed when creating the connection
	cluster.Destination = destination
	cluster.Version, err = greengage.Version(gphome)
	if err != nil {
		return nil, err
	}

	conn := cluster.Connection([]greengage.Option{greengage.Port(port), greengage.Database(database)}...)
	db, err := sql.Open("pgx", conn)
	if err != nil {
		return nil, err
	}

	return db, nil
}
