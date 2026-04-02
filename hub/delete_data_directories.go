// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package hub

import (
	"context"
	"sync"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/step"
	"github.com/GreengageDB/ggupgrade/upgrade"
	"github.com/GreengageDB/ggupgrade/utils/errorlist"
)

func DeleteCoordinatorAndPrimaryDataDirectories(streams step.OutStreams, agentConns []*idl.Connection, intermediate *greengage.Cluster) error {
	coordinatorErr := make(chan error)
	go func() {
		coordinatorErr <- upgrade.DeleteDirectories([]string{intermediate.CoordinatorDataDir()}, upgrade.PostgresFiles, streams)
	}()

	intermediateSegs := intermediate.SelectSegments(func(seg *greengage.SegConfig) bool {
		return seg.IsPrimary()
	})
	err := deleteDataDirectories(agentConns, intermediateSegs)
	err = errorlist.Append(err, <-coordinatorErr)

	return err
}

func deleteDataDirectories(agentConns []*idl.Connection, segConfigs greengage.SegConfigs) error {
	request := func(conn *idl.Connection) error {

		segs := segConfigs.Select(func(seg *greengage.SegConfig) bool {
			return seg.Hostname == conn.Hostname
		})

		if len(segs) == 0 {
			// This can happen if there are no segments matching the filter on a host
			return nil
		}

		req := new(idl.DeleteDataDirectoriesRequest)
		for _, seg := range segs {
			datadir := seg.DataDir
			req.Datadirs = append(req.Datadirs, datadir)
		}

		_, err := conn.AgentClient.DeleteDataDirectories(context.Background(), req)
		return err
	}

	return ExecuteRPC(agentConns, request)
}

func DeleteTargetTablespaces(streams step.OutStreams, agentConns []*idl.Connection, target *greengage.Cluster, intermediateCatalogVersion string, sourceTablespaces greengage.Tablespaces) error {
	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- DeleteTargetTablespacesOnCoordinator(streams, target, sourceTablespaces.GetCoordinatorTablespaces(), intermediateCatalogVersion)
	}()

	errs <- DeleteTargetTablespacesOnPrimaries(agentConns, target, sourceTablespaces, intermediateCatalogVersion)

	wg.Wait()
	close(errs)

	var err error
	for e := range errs {
		err = errorlist.Append(err, e)
	}

	return err
}

func DeleteTargetTablespacesOnCoordinator(streams step.OutStreams, target *greengage.Cluster, coordinatorTablespaces greengage.SegmentTablespaces, catalogVersion string) error {
	var dirs []string
	for _, tsInfo := range coordinatorTablespaces {
		if !tsInfo.GetUserDefined() {
			continue
		}

		path := upgrade.TablespacePath(tsInfo.GetLocation(), int32(target.Coordinator().DbID), target.Version.Major, catalogVersion)
		dirs = append(dirs, path)
	}

	return upgrade.DeleteTablespaceDirectories(streams, dirs)
}

func DeleteTargetTablespacesOnPrimaries(agentConns []*idl.Connection, target *greengage.Cluster, tablespaces greengage.Tablespaces, catalogVersion string) error {
	request := func(conn *idl.Connection) error {
		if target == nil {
			return nil
		}

		primaries := target.SelectSegments(func(seg *greengage.SegConfig) bool {
			return seg.IsOnHost(conn.Hostname) && seg.IsPrimary() && !seg.IsCoordinator()
		})

		if len(primaries) == 0 {
			return nil
		}

		var dirs []string
		for _, seg := range primaries {
			segTablespaces := tablespaces[int32(seg.DbID)]
			for _, tsInfo := range segTablespaces {
				if !tsInfo.GetUserDefined() {
					continue
				}

				path := upgrade.TablespacePath(tsInfo.GetLocation(), int32(seg.DbID), target.Version.Major, catalogVersion)
				dirs = append(dirs, path)
			}
		}

		req := &idl.DeleteTablespaceRequest{Dirs: dirs}
		_, err := conn.AgentClient.DeleteTablespaceDirectories(context.Background(), req)
		return err
	}

	return ExecuteRPC(agentConns, request)
}
