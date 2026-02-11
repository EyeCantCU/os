#!/bin/bash
# Start two 3-broker Kafka clusters in background:
# - BCFKS cluster on ports 19092-19097
# - PEM cluster on ports 29092-29097
set -o errexit -o nounset -o errtrace -o pipefail -x

export PATH="/usr/lib/kafka/bin:$PATH"

# Start BCFKS cluster
BCFKS_CLUSTER_ID=$(kafka-storage.sh random-uuid)
for i in 1 2 3; do
  kafka-storage.sh format -t "$BCFKS_CLUSTER_ID" -c /tmp/quorum-bcfks-${i}.properties
  kafka-server-start.sh /tmp/quorum-bcfks-${i}.properties > /tmp/quorum-bcfks-${i}.log 2>&1 &
done

# Start PEM cluster
PEM_CLUSTER_ID=$(kafka-storage.sh random-uuid)
for i in 1 2 3; do
  kafka-storage.sh format -t "$PEM_CLUSTER_ID" -c /tmp/quorum-pem-${i}.properties
  kafka-server-start.sh /tmp/quorum-pem-${i}.properties > /tmp/quorum-pem-${i}.log 2>&1 &
done
