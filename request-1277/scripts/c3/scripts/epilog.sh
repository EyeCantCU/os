#!/usr/bin/env bash

set -e

# Load libraries
. /c3/bin/libfs.sh
. /c3/bin/libcassandra.sh
. /c3/bin/libpackage.sh
. /c3/bin/libjava.sh
. /c3/bin/libpython.sh


Java11.all

install_package jemalloc
install_package iproute
install_package numactl
install_package findutils

# Load Cassandra environment variables
eval "$(cassandra_env)"

for dir in "${CASSANDRA_MUTABLE_DIR}" "$CASSANDRA_CONF_DIR" "$CASSANDRA_DATA_DIR" "$CASSANDRA_HISTORY_DIR" "$CASSANDRA_LOG_DIR" "$CASSANDRA_INITSCRIPTS_DIR"; do
    ensure_dir_exists "$dir"
done

Python3.install