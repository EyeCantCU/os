#!/bin/bash
# Wait for both 3-broker clusters to be ready.
# Fails with logs if any broker doesn't start.
set -o errexit -o nounset -o errtrace -o pipefail -x

# Wait for BCFKS cluster (ports 19092, 19094, 19096)
for i in 0 2 4; do
  port=$((BCFKS_CLUSTER_BASE_PORT + i))
  if ! wait-for-port --host=localhost $port; then
    echo "BCFKS broker on port $port failed to start"
    cat /tmp/quorum-bcfks-*.log
    exit 1
  fi
done

# Wait for PEM cluster (ports 29092, 29094, 29096)
for i in 0 2 4; do
  port=$((PEM_CLUSTER_BASE_PORT + i))
  if ! wait-for-port --host=localhost $port; then
    echo "PEM broker on port $port failed to start"
    cat /tmp/quorum-pem-*.log
    exit 1
  fi
done
