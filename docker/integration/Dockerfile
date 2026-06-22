# Runs the integration test suite against a real systemd instance.
#
# Integration tests talk to the actual systemd D-Bus API, which only exists
# when systemd is PID 1, so they cannot run in a normal container. This image
# boots systemd itself and the test suite is executed inside it via
# `docker exec` once it is up (see Makefile target test-integration-docker).
FROM debian:trixie-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        systemd \
        dbus \
        golang-go \
        ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && rm -f /lib/systemd/system/multi-user.target.wants/* \
             /etc/systemd/system/*.wants/* \
             /lib/systemd/system/local-fs.target.wants/* \
             /lib/systemd/system/sockets.target.wants/*udev* \
             /lib/systemd/system/sockets.target.wants/*initctl*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

STOPSIGNAL SIGRTMIN+3
ENTRYPOINT ["/lib/systemd/systemd"]
