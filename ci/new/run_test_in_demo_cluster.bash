#!/bin/bash

docker run -i --rm \
 -v "$(pwd)/logs:/logs" \
 "$IMAGE" bash -ex <<EOF
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

    bash ggupgrade/ci/new/collect_logs.bash ggdb6_src/gpAux/gpdemo/datadirs/ bash -c

    exit \$exit_code
EOF
