#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

[[ "${C3_TRACE:-false}" == "true" ]] && set -o xtrace

# Load libraries
. /c3/bin/libcassandra.sh
. /c3/bin/libnss.sh

# Load Cassandra environment variables
eval "$(cassandra_env)"

enable_nss_wrapper

if [[ "$*" = *"/run.sh"* ]]; then
    info "** Starting Cassandra setup **"
    /c3/scripts/setup.sh
    info "** Cassandra setup finished! **"
fi

exec "$@"
