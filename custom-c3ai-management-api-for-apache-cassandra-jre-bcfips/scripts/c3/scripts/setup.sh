#!/usr/bin/env bash
set -xtrace
set -o errexit
set -o nounset
set -o pipefail

[[ "${C3_TRACE:-false}" == "true" ]] && set -o xtrace

# Load Generic Libraries
. /c3/bin/libvalidations.sh
. /c3/bin/libos.sh
. /c3/bin/libcassandra.sh

if [[ $HOSTNAME =~ (.*)-0$ ]]; then
		echo "Setting node as password seeder"
		export CASSANDRA_PASSWORD_SEEDER=yes
else
		# Only node 0 will execute the startup initdb scripts
		export CASSANDRA_IGNORE_INITDB_SCRIPTS=1
fi

eval "$(cassandra_env)"

# Ensure Cassandra environment variables settings are valid
cassandra_validate
# Ensure Cassandra is stopped when this script ends.
trap "cassandra_stop" EXIT
# Ensure 'daemon' user exists when running as 'root'
am_i_root && ensure_user_exists "$DAEMON_USER" "$DAEMON_GROUP"

for dir in "${CASSANDRA_MUTABLE_DIR}" "$CASSANDRA_CONF_DIR" "$CASSANDRA_DATA_DIR" "$CASSANDRA_HISTORY_DIR" "$CASSANDRA_LOG_DIR" "$CASSANDRA_INITSCRIPTS_DIR" "$CASSANDRA_TMP_DIR"; do
    ensure_dir_exists "$dir"
#    chmod -R g+rwX "$dir"
done

# Ensure Cassandra is initialized
cassandra_initialize

# Allow running custom initialization scripts
if ! is_boolean_yes "$CASSANDRA_IGNORE_INITDB_SCRIPTS"; then
    cassandra_custom_init_scripts
fi
