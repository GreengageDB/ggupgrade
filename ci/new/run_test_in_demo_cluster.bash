#!/bin/bash

docker run -i --rm \
 -v "$(pwd)/logs:/logs" \
 gpdb7_ggupgrade:latest bash -ex <<EOF
    set -exuo pipefail
    gpdb_src/concourse/scripts/setup_gpadmin_user.bash
    set +e
    su gpadmin <<EOF1
        set -exuo pipefail
        source /usr/local/greengage-db-6X/greengage_path.sh
        cd /home/gpadmin/ggdb6_src/
        make create-demo-cluster WITH_MIRRORS=${WITH_MIRRORS:-true} WITH_STANDBY=${WITH_STANDBY:-true}

        source /usr/local/greengage-db-6X/greengage_path.sh
        source /home/gpadmin/ggdb6_src/gpAux/gpdemo/gpdemo-env.sh
        export GPHOME_SOURCE=/usr/local/greengage-db-6X/
        export GPHOME_TARGET=/usr/local/greengage-db-7X/
        export PGPORT=6000
        export PGHOST=/tmp/
        export PATH=\\\$PATH:/opt/go/bin:~/go/bin
        export GOPATH=~/go
        export ISOLATION2_PATH=/home/gpadmin/gpdb_src/src/test/isolation2

#       run test command passed to script
        cd /home/gpadmin/ggupgrade
        set +e
        $@
EOF1
    exit_code=\$?
    set -e

    params=(
      "./ d gpAdminLogs"
      "ggdb6_src/gpAux/gpdemo/datadirs/ d log"
      "ggdb6_src/gpAux/gpdemo/datadirs/ d pg_log"
      "ggupgrade/test/acceptance/pg_upgrade/6-to-7/ d results"
    )
    for param in "\${params[@]}"; do
      read -r path type name <<< "\$param"
      find \$path -name \$name -type \$type -exec tar -rf "/logs/\$name.tar" {} \;
    done

    cp ggupgrade/test/acceptance/pg_upgrade/6-to-7/non_upgradeable_tests/regression.diffs /logs/regression_non_upgradeable.diffs || true
    cp ggupgrade/test/acceptance/pg_upgrade/6-to-7/upgradeable_tests/source_cluster_regress/regression.diffs /logs/regression_upgradeable.diffs || true

    exit \$exit_code
EOF
