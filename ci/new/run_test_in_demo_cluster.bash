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
        pushd /home/gpadmin/ggdb6_src/
        make create-demo-cluster WITH_MIRRORS=${WITH_MIRRORS:-true} WITH_STANDBY=${WITH_STANDBY:-true}
        popd

        source /usr/local/greengage-db-6X/greengage_path.sh
        source /home/gpadmin/ggdb6_src/gpAux/gpdemo/gpdemo-env.sh

        source /home/gpadmin/ggupgrade/ci/new/common.bash
        export PGPORT=6000
#       run test command passed to script
        pushd /home/gpadmin/ggupgrade
        set +e
        $@
        exit_code=\\\$?
        set -e
        popd

        collect_logs ggdb6_src/gpAux/gpdemo/datadirs/ bash -c

        exit \\\$exit_code
EOF1
EOF
