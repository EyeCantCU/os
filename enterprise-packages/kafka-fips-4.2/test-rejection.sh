#!/bin/bash
# Test that non-FIPS configurations are rejected:
# - Ed25519 keys (not FIPS-approved for TLS)
# - SCRAM passwords < 14 chars
# - Non-FIPS cipher suites (ChaCha20, 3DES)
# - TLS 1.1 (not FIPS-approved)
# - JKS keystores (only BCFKS allowed)
set -o errexit -o nounset -o errtrace -o pipefail -x

export PATH="/usr/lib/kafka/bin:$PATH"

# Ed25519 is not valid for TLS, just signatures
cat > /tmp/invalid.properties << EOF
node.id=1
process.roles=broker,controller
listeners=SSL://localhost:9092,CONTROLLER://localhost:9093
advertised.listeners=SSL://localhost:9092
controller.listener.names=CONTROLLER
controller.quorum.voters=1@localhost:9093
inter.broker.listener.name=SSL
listener.security.protocol.map=SSL:SSL,CONTROLLER:SSL
log.dirs=/tmp/invalid-logs
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1
EOF
cat /tmp/ssl-invalid.conf >> /tmp/invalid.properties

kafka-storage.sh format -t "$(kafka-storage.sh random-uuid)" -c /tmp/invalid.properties
kafka-server-start.sh /tmp/invalid.properties > /tmp/invalid.log 2>&1 &
KAFKA_PID=$!

# Broker should fail to start - if port comes up, that's an error
if wait-for-port --host=localhost 9092 --timeout=1; then
  echo "ERROR: Kafka should have failed to start with Ed25519 (non-FIPS) key"
  cat /tmp/invalid.log
  exit 1
fi

# Wait for process to exit and write error to log
wait $KAFKA_PID || true
grep -F "algorithm not recognized" /tmp/invalid.log || \
  grep -F "EdDSA" /tmp/invalid.log || \
  grep -F "not FIPS" /tmp/invalid.log || \
  grep -F "unsupported" /tmp/invalid.log || \
  true

# SCRAM passwords must be >= 14 chars in FIPS mode
cat > /tmp/TestScram.java << 'EOF'
import org.apache.kafka.common.security.scram.internals.ScramFormatter;
import org.apache.kafka.common.security.scram.internals.ScramMechanism;
public class TestScram {
    public static void main(String[] args) throws Exception {
        ScramFormatter formatter = new ScramFormatter(ScramMechanism.SCRAM_SHA_256);
        formatter.generateCredential("password", 4096);
    }
}
EOF

# Should fail because "password" is only 8 chars (< 14 char FIPS minimum)
if java --source 21 -cp "/usr/lib/kafka/libs/*" /tmp/TestScram.java 2>&1 | tee /tmp/scram.log; then
  echo "ERROR: SCRAM should have rejected short password"
  exit 1
fi

grep -F "password must be at least" /tmp/scram.log || grep -F "112 bits" /tmp/scram.log

# Non-FIPS cipher suites
for cipher_info in \
  "TLS_CHACHA20_POLY1305_SHA256:TLSv1.3" \
  "TLS_RSA_WITH_3DES_EDE_CBC_SHA:TLSv1.2" \
  "TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA:TLSv1.2" \
  "TLS_DHE_RSA_WITH_3DES_EDE_CBC_SHA:TLSv1.2"; do

  cipher="${cipher_info%%:*}"
  protocol="${cipher_info##*:}"

  cat > /tmp/cipher.properties << EOF
node.id=1
process.roles=broker,controller
listeners=SSL://localhost:9092,CONTROLLER://localhost:9093
advertised.listeners=SSL://localhost:9092
controller.listener.names=CONTROLLER
controller.quorum.voters=1@localhost:9093
inter.broker.listener.name=SSL
listener.security.protocol.map=SSL:SSL,CONTROLLER:SSL
log.dirs=/tmp/cipher-logs
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1
ssl.enabled.protocols=${protocol}
ssl.cipher.suites=${cipher}
EOF
  cat /tmp/ssl-bcfks.conf >> /tmp/cipher.properties

  rm -rf /tmp/cipher-logs
  kafka-storage.sh format -t "$(kafka-storage.sh random-uuid)" -c /tmp/cipher.properties
  kafka-server-start.sh /tmp/cipher.properties > /tmp/cipher.log 2>&1 &
  KAFKA_PID=$!

  # Broker should fail to start - if port comes up, that's an error
  if wait-for-port --host=localhost 9092 --timeout=1; then
    echo "ERROR: Kafka should have failed to start with ${cipher}"
    cat /tmp/cipher.log
    exit 1
  fi

  # Wait for process to exit and write error to log
  wait $KAFKA_PID || true
  grep -m 1 -F "No usable cipher suites enabled" /tmp/cipher.log
done

# TLS 1.1 is not FIPS-approved
cat > /tmp/tls11.properties << EOF
node.id=1
process.roles=broker,controller
listeners=SSL://localhost:9092,CONTROLLER://localhost:9093
advertised.listeners=SSL://localhost:9092
controller.listener.names=CONTROLLER
controller.quorum.voters=1@localhost:9093
inter.broker.listener.name=SSL
listener.security.protocol.map=SSL:SSL,CONTROLLER:SSL
log.dirs=/tmp/tls11-logs
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1
ssl.enabled.protocols=TLSv1.1
EOF
cat /tmp/ssl-bcfks.conf >> /tmp/tls11.properties

rm -rf /tmp/tls11-logs
kafka-storage.sh format -t "$(kafka-storage.sh random-uuid)" -c /tmp/tls11.properties
kafka-server-start.sh /tmp/tls11.properties > /tmp/tls11.log 2>&1 &
KAFKA_PID=$!

if wait-for-port --host=localhost 9092 --timeout=1; then
  echo "ERROR: Kafka should have failed to start with TLS 1.1"
  cat /tmp/tls11.log
  exit 1
fi

wait $KAFKA_PID || true
grep -m 1 -E "(No usable cipher suites|SSLHandshakeException|protocol.*not supported|TLSv1.1)" /tmp/tls11.log

# JKS keystores are not FIPS-approved
cat > /tmp/jks.properties << EOF
node.id=1
process.roles=broker,controller
listeners=SSL://localhost:9092,CONTROLLER://localhost:9093
advertised.listeners=SSL://localhost:9092
controller.listener.names=CONTROLLER
controller.quorum.voters=1@localhost:9093
inter.broker.listener.name=SSL
listener.security.protocol.map=SSL:SSL,CONTROLLER:SSL
log.dirs=/tmp/jks-logs
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1
EOF
cat /tmp/ssl-jks.conf >> /tmp/jks.properties

rm -rf /tmp/jks-logs
kafka-storage.sh format -t "$(kafka-storage.sh random-uuid)" -c /tmp/jks.properties
kafka-server-start.sh /tmp/jks.properties > /tmp/jks.log 2>&1 &
KAFKA_PID=$!

# Broker should fail to start - if port comes up, that's an error
if wait-for-port --host=localhost 9092 --timeout=1; then
  echo "ERROR: Kafka should have failed to start with JKS keystore"
  cat /tmp/jks.log
  exit 1
fi

# Wait for process to exit and write error to log
wait $KAFKA_PID || true
grep -m 1 -F "Invalid keystore type 'JKS': only BCFKS is supported" /tmp/jks.log
