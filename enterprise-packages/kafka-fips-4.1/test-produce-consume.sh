#!/bin/bash
# Test end-to-end produce/consume with both 3-broker clusters.
# Verifies inter-broker communication, Raft consensus, and topic replication over FIPS TLS.
set -o errexit -o nounset -o errtrace -o pipefail -x

export PATH="/usr/lib/kafka/bin:$PATH"

# BCFKS cluster
BASE_PORT=${BCFKS_CLUSTER_BASE_PORT}
SSL_PORTS="localhost:${BASE_PORT},localhost:$((BASE_PORT+2)),localhost:$((BASE_PORT+4))"
TOPIC_NAME="quorum-bcfks-test-$$"

kafka-topics.sh --create \
  --topic "${TOPIC_NAME}" \
  --partitions 3 \
  --replication-factor 3 \
  --bootstrap-server "${SSL_PORTS}" \
  --command-config /tmp/client-bcfks.properties

kafka-topics.sh --describe \
  --topic "${TOPIC_NAME}" \
  --bootstrap-server localhost:${BASE_PORT} \
  --command-config /tmp/client-bcfks.properties | grep -F "ReplicationFactor: 3"

echo "Hello Quorum BCFKS" | kafka-console-producer.sh \
  --bootstrap-server "${SSL_PORTS}" \
  --topic "${TOPIC_NAME}" \
  --producer.config /tmp/client-bcfks.properties

consumed=$(timeout 10 kafka-console-consumer.sh \
  --bootstrap-server localhost:$((BASE_PORT+2)) \
  --topic "${TOPIC_NAME}" \
  --from-beginning \
  --max-messages 1 \
  --consumer.config /tmp/client-bcfks.properties)

test "$consumed" = "Hello Quorum BCFKS"

# PEM cluster
BASE_PORT=${PEM_CLUSTER_BASE_PORT}
SSL_PORTS="localhost:${BASE_PORT},localhost:$((BASE_PORT+2)),localhost:$((BASE_PORT+4))"
TOPIC_NAME="quorum-pem-test-$$"

kafka-topics.sh --create \
  --topic "${TOPIC_NAME}" \
  --partitions 3 \
  --replication-factor 3 \
  --bootstrap-server "${SSL_PORTS}" \
  --command-config /tmp/client-pem.properties

kafka-topics.sh --describe \
  --topic "${TOPIC_NAME}" \
  --bootstrap-server localhost:${BASE_PORT} \
  --command-config /tmp/client-pem.properties | grep -F "ReplicationFactor: 3"

echo "Hello Quorum PEM" | kafka-console-producer.sh \
  --bootstrap-server "${SSL_PORTS}" \
  --topic "${TOPIC_NAME}" \
  --producer.config /tmp/client-pem.properties

consumed=$(timeout 10 kafka-console-consumer.sh \
  --bootstrap-server localhost:$((BASE_PORT+2)) \
  --topic "${TOPIC_NAME}" \
  --from-beginning \
  --max-messages 1 \
  --consumer.config /tmp/client-pem.properties)

test "$consumed" = "Hello Quorum PEM"
