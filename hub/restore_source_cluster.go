// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package hub

import (
	"context"
	"os"
	"sync"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/step"
	"github.com/GreengageDB/ggupgrade/upgrade"
	"github.com/GreengageDB/ggupgrade/utils/errorlist"
	"github.com/GreengageDB/ggupgrade/utils/rsync"
)

func RsyncCoordinatorAndPrimaries(stream step.OutStreams, agentConns []*idl.Connection, source *greengage.Cluster) error {
	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- RsyncCoordinator(stream, source.Standby(), source.Coordinator())
	}()

	errs <- RsyncPrimaries(agentConns, source)

	wg.Wait()
	close(errs)

	var err error
	for e := range errs {
		err = errorlist.Append(err, e)
	}

	return err
}

func RsyncCoordinatorAndPrimariesTablespaces(stream step.OutStreams, agentConns []*idl.Connection, source *greengage.Cluster) error {
	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- RsyncCoordinatorTablespaces(stream, source.StandbyHostname(), source.Tablespaces[int32(source.Coordinator().DbID)], source.Tablespaces[int32(source.Standby().DbID)])
	}()

	errs <- RsyncPrimariesTablespaces(agentConns, source, source.Tablespaces)

	wg.Wait()
	close(errs)

	var err error
	for e := range errs {
		err = errorlist.Append(err, e)
	}

	return err
}

func Recoverseg(stream step.OutStreams, cluster *greengage.Cluster, useHbaHostnames bool) error {
	args := []string{"-a"}
	if useHbaHostnames {
		args = append(args, "--hba-hostnames")
	}

	return cluster.RunGreengageCmd(stream, "gprecoverseg", args...)
}

func RsyncCoordinator(stream step.OutStreams, standby greengage.SegConfig, coordinator greengage.SegConfig) error {
	opts := []rsync.Option{
		rsync.WithSources(standby.DataDir + string(os.PathSeparator)),
		rsync.WithSourceHost(standby.Hostname),
		rsync.WithDestination(coordinator.DataDir),
		rsync.WithOptions(rsync.Options...),
		rsync.WithExcludedFiles(rsync.Excludes...),
		rsync.WithStream(stream),
	}

	return rsync.Rsync(opts...)
}

func RsyncCoordinatorTablespaces(stream step.OutStreams, standbyHostname string, coordinatorTablespaces greengage.SegmentTablespaces, standbyTablespaces greengage.SegmentTablespaces) error {
	for oid, coordinatorTsInfo := range coordinatorTablespaces {
		if !coordinatorTsInfo.GetUserDefined() {
			continue
		}

		opts := []rsync.Option{
			rsync.WithSourceHost(standbyHostname),
			rsync.WithSources(standbyTablespaces[oid].GetLocation() + string(os.PathSeparator)),
			rsync.WithDestination(coordinatorTsInfo.GetLocation()),
			rsync.WithOptions(rsync.Options...),
			rsync.WithStream(stream),
		}

		err := rsync.Rsync(opts...)
		if err != nil {
			return err
		}
	}

	return nil
}

func RsyncPrimaries(agentConns []*idl.Connection, source *greengage.Cluster) error {
	request := func(conn *idl.Connection) error {
		mirrors := source.SelectSegments(func(seg *greengage.SegConfig) bool {
			return seg.IsOnHost(conn.Hostname) && !seg.IsStandby() && seg.IsMirror()
		})

		if len(mirrors) == 0 {
			return nil
		}

		var opts []*idl.RsyncRequest_RsyncOptions
		for _, mirror := range mirrors {
			opt := &idl.RsyncRequest_RsyncOptions{
				Sources:         []string{mirror.DataDir + string(os.PathSeparator)},
				DestinationHost: source.Primaries[mirror.ContentID].Hostname,
				Destination:     source.Primaries[mirror.ContentID].DataDir,
				Options:         rsync.Options,
				ExcludedFiles:   rsync.Excludes,
			}
			opts = append(opts, opt)
		}

		req := &idl.RsyncRequest{Options: opts}
		_, err := conn.AgentClient.RsyncDataDirectories(context.Background(), req)
		return err
	}

	return ExecuteRPC(agentConns, request)
}

func RsyncPrimariesTablespaces(agentConns []*idl.Connection, source *greengage.Cluster, tablespaces greengage.Tablespaces) error {
	request := func(conn *idl.Connection) error {
		mirrors := source.SelectSegments(func(seg *greengage.SegConfig) bool {
			return seg.IsOnHost(conn.Hostname) && !seg.IsStandby() && seg.IsMirror()
		})

		if len(mirrors) == 0 {
			return nil
		}

		var opts []*idl.RsyncRequest_RsyncOptions
		for _, mirror := range mirrors {
			primary := source.Primaries[mirror.ContentID]

			primaryTablespaces := tablespaces[int32(primary.DbID)]
			mirrorTablespaces := tablespaces[int32(mirror.DbID)]
			for oid, mirrorTsInfo := range mirrorTablespaces {
				if !mirrorTsInfo.GetUserDefined() {
					continue
				}

				opt := &idl.RsyncRequest_RsyncOptions{
					Sources:         []string{mirrorTsInfo.GetLocation() + string(os.PathSeparator)},
					DestinationHost: primary.Hostname,
					Destination:     primaryTablespaces[oid].GetLocation(),
					Options:         rsync.Options,
					ExcludedFiles:   rsync.Excludes,
				}
				opts = append(opts, opt)
			}
		}

		req := &idl.RsyncRequest{Options: opts}
		_, err := conn.AgentClient.RsyncTablespaceDirectories(context.Background(), req)
		return err
	}

	return ExecuteRPC(agentConns, request)
}

func RestoreCoordinatorAndPrimariesPgControl(streams step.OutStreams, agentConns []*idl.Connection, source *greengage.Cluster) error {
	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- upgrade.RestorePgControl(source.CoordinatorDataDir(), streams)
	}()

	errs <- restorePrimariesPgControl(agentConns, source)

	wg.Wait()
	close(errs)

	var err error
	for e := range errs {
		err = errorlist.Append(err, e)
	}

	return err
}

func restorePrimariesPgControl(agentConns []*idl.Connection, source *greengage.Cluster) error {
	request := func(conn *idl.Connection) error {
		primaries := source.SelectSegments(func(seg *greengage.SegConfig) bool {
			return seg.IsOnHost(conn.Hostname) && !seg.IsStandby() && seg.IsPrimary()
		})

		if len(primaries) == 0 {
			return nil
		}

		var dataDirs []string
		for _, primary := range primaries {
			dataDirs = append(dataDirs, primary.DataDir)
		}

		req := &idl.RestorePgControlRequest{
			Datadirs: dataDirs,
		}

		_, err := conn.AgentClient.RestorePrimariesPgControl(context.Background(), req)
		return err
	}

	return ExecuteRPC(agentConns, request)
}
