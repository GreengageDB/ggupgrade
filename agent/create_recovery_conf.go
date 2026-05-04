// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/utils/errorlist"
)

func (s *Server) CreateRecoveryConf(ctx context.Context, req *idl.CreateRecoveryConfRequest) (*idl.CreateRecoveryConfReply, error) {
	log.Print("starting create recovery.conf")

	err := createRecoveryConf(req.GetConnections())
	if err != nil {
		return &idl.CreateRecoveryConfReply{}, err
	}

	return &idl.CreateRecoveryConfReply{}, nil
}

func createRecoveryConf(connReqs []*idl.CreateRecoveryConfRequest_Connection) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(connReqs))

	for _, connReq := range connReqs {
		wg.Add(1)

		go func(connReq *idl.CreateRecoveryConfRequest_Connection) {
			defer wg.Done()

			common_config := fmt.Sprintf(`
primary_conninfo = 'user=%s host=%s port=%d sslmode=disable sslcompression=1 krbsrvname=postgres application_name=gp_walreceiver'
primary_slot_name = 'internal_wal_replication_slot'`, connReq.GetUser(), connReq.GetPrimaryHost(), connReq.GetPrimaryPort())

			switch {
			case connReq.GetTargetMajorVersion() == 6:
				config := `standby_mode = 'on'` + common_config
				err := os.WriteFile(filepath.Join(connReq.GetMirrorDataDir(), "recovery.conf"), []byte(config), 0644)
				if err != nil {
					errs <- err
					return
				}
			case connReq.GetTargetMajorVersion() == 7:
				f, err := os.OpenFile(filepath.Join(connReq.GetMirrorDataDir(), "postgresql.auto.conf"), os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					errs <- err
					return
				}

				defer func() {
					if cErr := f.Close(); cErr != nil {
						errs <- errorlist.Append(err, cErr)
					}
				}()

				_, err = f.WriteString(common_config)
				if err != nil {
					errs <- err
					return
				}

				fSignal, err := os.Create(filepath.Join(connReq.GetMirrorDataDir(), "standby.signal"))
				if err != nil {
					errs <- err
					return
				}

				defer func() {
					if cErr := fSignal.Close(); cErr != nil {
						errs <- errorlist.Append(err, cErr)
					}
				}()
			default:
				errs <- fmt.Errorf("Unexpected target version")
			}
		}(connReq)
	}

	wg.Wait()
	close(errs)

	var err error
	for e := range errs {
		err = errorlist.Append(err, e)
	}

	return err
}
