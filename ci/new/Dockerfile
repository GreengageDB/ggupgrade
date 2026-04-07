FROM ghcr.io/greengagedb/greengage/ggdb7_ubuntu:latest

RUN wget https://golang.org/dl/go1.20.5.linux-amd64.tar.gz -q -O - | tar -C /opt -xz; \
    apt update; \
    apt install python2 python2-dev python-pip -y; \
    ln -sf /bin/python2 /bin/python; \
    python -m pip install future==0.16;

COPY . /home/gpadmin/ggupgrade
