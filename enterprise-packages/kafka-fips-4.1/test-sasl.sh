#!/bin/bash
# Test SASL_SSL with SCRAM-SHA-256 authentication.
# Verifies SCRAM auth works with FIPS-compliant password (14+ chars).
# Uses a separate single broker (SASL requires different listener config).
set -o errexit -o nounset -o errtrace -o pipefail -x

export PATH="/usr/lib/kafka/bin:$PATH"

SCRAM_PASSWORD="FipsCompliantPass123"

cat > /tmp/sasl.properties << EOF
node.id=1
process.roles=broker,controller
listeners=SASL_SSL://localhost:9092,CONTROLLER://localhost:9093
advertised.listeners=SASL_SSL://localhost:9092
controller.listener.names=CONTROLLER
controller.quorum.voters=1@localhost:9093
inter.broker.listener.name=SASL_SSL
listener.security.protocol.map=SASL_SSL:SASL_SSL,CONTROLLER:SSL
log.dirs=/tmp/sasl-logs
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1
sasl.enabled.mechanisms=SCRAM-SHA-256
sasl.mechanism.inter.broker.protocol=SCRAM-SHA-256
listener.name.sasl_ssl.scram-sha-256.sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required username="admin" password="${SCRAM_PASSWORD}";
EOF
cat /tmp/ssl-bcfks.conf >> /tmp/sasl.properties

kafka-storage.sh format -t "$(kafka-storage.sh random-uuid)" \
  -c /tmp/sasl.properties \
  --add-scram "SCRAM-SHA-256=[name=admin,password=${SCRAM_PASSWORD}]"

kafka-server-start.sh /tmp/sasl.properties > /tmp/sasl.log 2>&1 &

if ! wait-for-port --host=localhost 9092; then
  echo "Kafka SASL_SSL failed to start:"
  cat /tmp/sasl.log
  exit 1
fi

cp /tmp/sasl-client-base.properties /tmp/sasl-client.properties
echo "sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required username=\"admin\" password=\"${SCRAM_PASSWORD}\";" >> /tmp/sasl-client.properties

TOPIC_NAME="scram-test-$$"

kafka-topics.sh --create \
  --topic "${TOPIC_NAME}" \
  --bootstrap-server localhost:9092 \
  --command-config /tmp/sasl-client.properties

echo "Hello SCRAM" | kafka-console-producer.sh \
  --bootstrap-server localhost:9092 \
  --topic "${TOPIC_NAME}" \
  --producer.config /tmp/sasl-client.properties

consumed=$(timeout 10 kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic "${TOPIC_NAME}" \
  --from-beginning \
  --max-messages 1 \
  --consumer.config /tmp/sasl-client.properties)

test "$consumed" = "Hello SCRAM"
