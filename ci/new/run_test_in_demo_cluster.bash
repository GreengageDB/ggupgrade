#!/bin/bash

docker run -i --rm \
 -v "$(pwd)/logs:/logs" \
 "$IMAGE" bash -ex <<EOF
    set -exuo pipefail
    gpdb_src/concourse/scripts/setup_gpadmin_user.bash
    chown -R gpadmin:gpadmin /logs

    su gpadmin <<EOF1
        set -exuo pipefail
        source /usr/local/greengage-db-6X/greengage_path.sh
        pushd ggdb6_src
        make create-demo-cluster WITH_MIRRORS=${WITH_MIRRORS:-true} WITH_STANDBY=${WITH_STANDBY:-true}
        popd

        source /usr/local/greengage-db-6X/greengage_path.sh
        source ggdb6_src/gpAux/gpdemo/gpdemo-env.sh

        source ggupgrade/ci/new/common.bash
        export PGPORT=6000
#       run test command passed to script
        pushd ggupgrade
        set +e
        $@
        exit_code=\\\$?
        set -e
        popd

        collect_logs ggdb6_src/gpAux/gpdemo/datadirs/ bash -c

        exit \\\$exit_code
EOF1
EOF
