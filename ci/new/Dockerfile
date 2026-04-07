ARG GGDB6_IMAGE=ghcr.io/greengagedb/greengage/ggdb6_ubuntu:latest
ARG GGDB7_IMAGE=ghcr.io/greengagedb/greengage/ggdb7_ubuntu:latest

FROM $GGDB6_IMAGE AS ggdb6

RUN source gpdb_src/concourse/scripts/common.bash; \
    install_and_configure_gpdb;

FROM $GGDB7_IMAGE

COPY --from=ggdb6 /home/gpadmin/gpdb_src /home/gpadmin/ggdb6_src
COPY --from=ggdb6 /usr/local/greengage-db-devel /usr/local/greengage-db-6X

RUN wget https://golang.org/dl/go1.20.5.linux-amd64.tar.gz -q -O - | tar -C /opt -xz; \
    apt update; \
    apt install python2 python2-dev python-pip -y; \
    ln -sf /bin/python2 /bin/python; \
    python -m pip install future==0.16;

COPY . /home/gpadmin/ggupgrade
