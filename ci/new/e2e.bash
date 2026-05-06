#!/bin/bash

set -eux -o pipefail

MODE=$1
WITH_MIRRORS="${WITH_MIRRORS:-true}"
WITH_STANDBY="${WITH_STANDBY:-true}"

function load_dump() {
    echo "Loading SQL Dump"
    psql -d postgres -f ci/new/basic_sql_dump.sql &> sql_load.log
    echo "SQL Dump load complete"
}

load_dump

gpcheckcat -A

ggupgrade initialize \
          --non-interactive \
          --target-gphome $GPHOME_TARGET \
          --source-gphome $GPHOME_SOURCE \
          --source-master-port $PGPORT \
          --mode $MODE \
          --temp-port-range 6020-6040 \
          --disk-free-ratio 0

ggupgrade execute --non-interactive --skip-pg-upgrade-checks
ggupgrade finalize --non-interactive

if [ "${WITH_MIRRORS}" == "true" ] && [ "${WITH_STANDBY}" == "true" ]; then
    source testutils/validate_mirrors_and_standby/validate_mirrors_and_standby.bash
    validate_mirrors_and_standby /usr/local/greengage-db-7X coordinator 5432
fi
