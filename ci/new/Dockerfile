ARG GGDB6_IMAGE=ghcr.io/greengagedb/greengage/ggdb6_ubuntu:latest
ARG GGDB7_IMAGE=ghcr.io/greengagedb/greengage/ggdb7_ubuntu:latest

FROM $GGDB6_IMAGE AS ggdb6

RUN set -eux; \
    source gpdb_src/concourse/scripts/common.bash; \
    install_and_configure_gpdb

FROM $GGDB7_IMAGE

COPY --from=ggdb6 --chown=1000:1000 /home/gpadmin/gpdb_src ggdb6_src
COPY --from=ggdb6 --chown=1000:1000 /usr/local/greengage-db-devel /usr/local/greengage-db-6X

ENV PATH=$PATH:/opt/go/bin:/home/gpadmin/go/bin \
    GOPATH=/home/gpadmin/go \
    GPHOME_SOURCE=/usr/local/greengage-db-6X \
    GPHOME_TARGET=/usr/local/greengage-db-7X \
    PGHOST=/tmp \
    ISOLATION2_PATH=/home/gpadmin/gpdb_src/src/test/isolation2

RUN set -eux; \
    export CONFIGURE_FLAGS=; \
    source gpdb_src/concourse/scripts/common.bash; \
    install_and_configure_gpdb; \
    mv /usr/local/greengage-db-devel /usr/local/greengage-db-7X; \
    chown -R 1000:1000 /usr/local/greengage-db-7X

RUN set -eux; \
    source /usr/local/greengage-db-7X/greengage_path.sh; \
    make -C gpdb_src/src/test/isolation2 install

RUN set -eux; \
    wget https://golang.org/dl/go1.20.5.linux-amd64.tar.gz -q -O - | tar -C /opt -xz; \
    apt update; \
    . /etc/os-release; \
    if [ "$VERSION_ID" = "22.04" ]; then \
      apt install python2 python2-dev python-pip -y; \
      ln -sf /bin/python2 /bin/python; \
      python -m pip install future==0.16; \
    else \
      apt install python-is-python3; \
      python -m pip install future==1.0.0; \
    fi

COPY . ggupgrade

RUN set -eux; \
    mkdir -p go/bin; \
    chown -R 1000:1000 go; \
    cd ggupgrade; \
    make; \
    make install
