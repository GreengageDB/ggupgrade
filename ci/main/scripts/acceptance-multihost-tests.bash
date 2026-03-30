#!/bin/bash
# Copyright (c) 2017-2023 VMware, Inc. or its affiliates
# SPDX-License-Identifier: Apache-2.0

set -eux -o pipefail

function run_migration_scripts_and_tests() {
    time ssh cdw '
        set -eux -o pipefail

        export TERM=linux
        export PATH=$PATH:/usr/local/go/bin
        export GOFLAGS="-mod=readonly" # do not update dependencies during build

        source ggupgrade_src/ci/main/scripts/environment.bash
        source "${GPHOME_SOURCE}"/greengage_path.sh

        echo "Running data migration scripts to ensure a clean cluster..."
        ggupgrade generate --non-interactive --gphome "$GPHOME_SOURCE" --port "$PGPORT"
        ggupgrade apply    --non-interactive --gphome "$GPHOME_SOURCE" --port "$PGPORT" --phase initialize

        cd ggupgrade_src
        go test --cover -count=1 -timeout 30m -v -run "^TestRevert$" ./test/acceptance/ggupgrade
  '
}

main() {
    echo "Enabling ssh to cluster..."
    ./ccp_src/scripts/setup_ssh_to_cluster.sh

    echo "Installing ggupgrade_src on cdw..."
    scp -rpq ggupgrade_src gpadmin@cdw:/home/gpadmin

    echo "Running data migration scripts and tests..."
    run_migration_scripts_and_tests
}

main
