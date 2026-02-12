#!/bin/bash
# Create all SSL credentials and configs for FIPS testing:
# - BCFKS keystores (FIPS-approved)
# - PEM certificates (RSA 2048-bit)
# - JKS keystores (for rejection testing)
# - Invalid Ed25519 certs (for rejection testing)
# - Cluster configs for 3-broker BCFKS and PEM clusters
set -o errexit -o nounset -o errtrace -o pipefail -x

export PATH="/usr/lib/kafka/bin:$PATH"

# Invalid PEM (Ed25519 is not FIPS-approved)
openssl genpkey -algorithm ED25519 -out /tmp/invalid.key
openssl req -new -x509 -key /tmp/invalid.key -out /tmp/invalid.crt -days 365 \
  -subj "/CN=localhost"
INVALID_KEY_PEM=$(awk 'NF {sub(/\r/, ""); printf "%s\\n",$0;}' /tmp/invalid.key)
INVALID_CERT_PEM=$(awk 'NF {sub(/\r/, ""); printf "%s\\n",$0;}' /tmp/invalid.crt)

# Valid PEM (RSA 2048-bit)
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out /tmp/kafka.key
openssl req -new -x509 -key /tmp/kafka.key -out /tmp/kafka.crt -days 365 \
  -subj "/CN=localhost"
KEY_PEM=$(awk 'NF {sub(/\r/, ""); printf "%s\\n",$0;}' /tmp/kafka.key)
CERT_PEM=$(awk 'NF {sub(/\r/, ""); printf "%s\\n",$0;}' /tmp/kafka.crt)

# BCFKS keystore/truststore
KEYSTORE_PASSWORD="KafkaKeyStoreP@ss123"
TRUSTSTORE_PASSWORD="KafkaTrustStoreP@ss123"

keytool -genkeypair -alias kafka -keyalg RSA -keysize 2048 -validity 365 \
  -dname "CN=localhost" \
  -keystore /tmp/kafka.keystore.bcfks -storetype BCFKS \
  -storepass "${KEYSTORE_PASSWORD}" -keypass "${KEYSTORE_PASSWORD}" \
  -providername BCFIPS \
  -provider org.bouncycastle.jcajce.provider.BouncyCastleFipsProvider

keytool -exportcert -alias kafka \
  -keystore /tmp/kafka.keystore.bcfks -storetype BCFKS \
  -storepass "${KEYSTORE_PASSWORD}" \
  -providername BCFIPS \
  -provider org.bouncycastle.jcajce.provider.BouncyCastleFipsProvider \
  -file /tmp/kafka.der

keytool -importcert -alias kafka -file /tmp/kafka.der \
  -keystore /tmp/kafka.truststore.bcfks -storetype BCFKS \
  -storepass "${TRUSTSTORE_PASSWORD}" -noprompt \
  -providername BCFIPS \
  -provider org.bouncycastle.jcajce.provider.BouncyCastleFipsProvider

# JKS keystore (not FIPS-approved) for rejection testing
# Use -J flags to bypass FIPS security properties
JKS_PASSWORD="JksPassword123!"
keytool -genkeypair -alias kafka -keyalg RSA -keysize 2048 -validity 365 \
  -dname "CN=localhost" \
  -keystore /tmp/kafka.keystore.jks -storetype JKS \
  -storepass "${JKS_PASSWORD}" -keypass "${JKS_PASSWORD}" \
  -J-Djava.security.properties=/dev/null \
  -J-Dorg.bouncycastle.fips.approved_only=false

keytool -exportcert -alias kafka \
  -keystore /tmp/kafka.keystore.jks -storetype JKS \
  -storepass "${JKS_PASSWORD}" \
  -J-Djava.security.properties=/dev/null \
  -J-Dorg.bouncycastle.fips.approved_only=false \
  -file /tmp/kafka-jks.der

keytool -importcert -alias kafka -file /tmp/kafka-jks.der \
  -keystore /tmp/kafka.truststore.jks -storetype JKS \
  -storepass "${JKS_PASSWORD}" -noprompt \
  -J-Djava.security.properties=/dev/null \
  -J-Dorg.bouncycastle.fips.approved_only=false

# Invalid PEM config (for rejection test)
tee /tmp/ssl-invalid.conf << EOF
ssl.keystore.type=PEM
ssl.keystore.certificate.chain=${INVALID_CERT_PEM}
ssl.keystore.key=${INVALID_KEY_PEM}
ssl.truststore.type=PEM
ssl.truststore.certificates=${INVALID_CERT_PEM}
ssl.client.auth=none
ssl.endpoint.identification.algorithm=
listener.name.controller.ssl.keystore.type=PEM
listener.name.controller.ssl.keystore.certificate.chain=${INVALID_CERT_PEM}
listener.name.controller.ssl.keystore.key=${INVALID_KEY_PEM}
listener.name.controller.ssl.truststore.type=PEM
listener.name.controller.ssl.truststore.certificates=${INVALID_CERT_PEM}
controller.quorum.ssl.keystore.type=PEM
controller.quorum.ssl.keystore.certificate.chain=${INVALID_CERT_PEM}
controller.quorum.ssl.keystore.key=${INVALID_KEY_PEM}
controller.quorum.ssl.truststore.type=PEM
controller.quorum.ssl.truststore.certificates=${INVALID_CERT_PEM}
EOF

# Valid PEM config
tee /tmp/ssl-pem.conf << EOF
ssl.keystore.type=PEM
ssl.keystore.certificate.chain=${CERT_PEM}
ssl.keystore.key=${KEY_PEM}
ssl.truststore.type=PEM
ssl.truststore.certificates=${CERT_PEM}
ssl.client.auth=none
ssl.endpoint.identification.algorithm=
listener.name.controller.ssl.keystore.type=PEM
listener.name.controller.ssl.keystore.certificate.chain=${CERT_PEM}
listener.name.controller.ssl.keystore.key=${KEY_PEM}
listener.name.controller.ssl.truststore.type=PEM
listener.name.controller.ssl.truststore.certificates=${CERT_PEM}
controller.quorum.ssl.keystore.type=PEM
controller.quorum.ssl.keystore.certificate.chain=${CERT_PEM}
controller.quorum.ssl.keystore.key=${KEY_PEM}
controller.quorum.ssl.truststore.type=PEM
controller.quorum.ssl.truststore.certificates=${CERT_PEM}
EOF

# BCFKS config
tee /tmp/ssl-bcfks.conf << EOF
ssl.keystore.type=BCFKS
ssl.keystore.location=/tmp/kafka.keystore.bcfks
ssl.keystore.password=${KEYSTORE_PASSWORD}
ssl.key.password=${KEYSTORE_PASSWORD}
ssl.truststore.type=BCFKS
ssl.truststore.location=/tmp/kafka.truststore.bcfks
ssl.truststore.password=${TRUSTSTORE_PASSWORD}
ssl.client.auth=none
ssl.endpoint.identification.algorithm=
listener.name.controller.ssl.keystore.type=BCFKS
listener.name.controller.ssl.keystore.location=/tmp/kafka.keystore.bcfks
listener.name.controller.ssl.keystore.password=${KEYSTORE_PASSWORD}
listener.name.controller.ssl.key.password=${KEYSTORE_PASSWORD}
listener.name.controller.ssl.truststore.type=BCFKS
listener.name.controller.ssl.truststore.location=/tmp/kafka.truststore.bcfks
listener.name.controller.ssl.truststore.password=${TRUSTSTORE_PASSWORD}
controller.quorum.ssl.keystore.type=BCFKS
controller.quorum.ssl.keystore.location=/tmp/kafka.keystore.bcfks
controller.quorum.ssl.keystore.password=${KEYSTORE_PASSWORD}
controller.quorum.ssl.key.password=${KEYSTORE_PASSWORD}
controller.quorum.ssl.truststore.type=BCFKS
controller.quorum.ssl.truststore.location=/tmp/kafka.truststore.bcfks
controller.quorum.ssl.truststore.password=${TRUSTSTORE_PASSWORD}
EOF

# JKS config (for rejection test)
tee /tmp/ssl-jks.conf << EOF
ssl.keystore.type=JKS
ssl.keystore.location=/tmp/kafka.keystore.jks
ssl.keystore.password=${JKS_PASSWORD}
ssl.key.password=${JKS_PASSWORD}
ssl.truststore.type=JKS
ssl.truststore.location=/tmp/kafka.truststore.jks
ssl.truststore.password=${JKS_PASSWORD}
ssl.client.auth=none
ssl.endpoint.identification.algorithm=
listener.name.controller.ssl.keystore.type=JKS
listener.name.controller.ssl.keystore.location=/tmp/kafka.keystore.jks
listener.name.controller.ssl.keystore.password=${JKS_PASSWORD}
listener.name.controller.ssl.key.password=${JKS_PASSWORD}
listener.name.controller.ssl.truststore.type=JKS
listener.name.controller.ssl.truststore.location=/tmp/kafka.truststore.jks
listener.name.controller.ssl.truststore.password=${JKS_PASSWORD}
controller.quorum.ssl.keystore.type=JKS
controller.quorum.ssl.keystore.location=/tmp/kafka.keystore.jks
controller.quorum.ssl.keystore.password=${JKS_PASSWORD}
controller.quorum.ssl.key.password=${JKS_PASSWORD}
controller.quorum.ssl.truststore.type=JKS
controller.quorum.ssl.truststore.location=/tmp/kafka.truststore.jks
controller.quorum.ssl.truststore.password=${JKS_PASSWORD}
EOF

# Client PEM config
tee /tmp/client-pem.properties << EOF
security.protocol=SSL
ssl.truststore.type=PEM
ssl.truststore.certificates=${CERT_PEM}
ssl.endpoint.identification.algorithm=
EOF

# Client BCFKS config
tee /tmp/client-bcfks.properties << EOF
security.protocol=SSL
ssl.truststore.type=BCFKS
ssl.truststore.location=/tmp/kafka.truststore.bcfks
ssl.truststore.password=${TRUSTSTORE_PASSWORD}
ssl.endpoint.identification.algorithm=
EOF

# SASL client base config
tee /tmp/sasl-client-base.properties << EOF
security.protocol=SASL_SSL
sasl.mechanism=SCRAM-SHA-256
ssl.truststore.type=BCFKS
ssl.truststore.location=/tmp/kafka.truststore.bcfks
ssl.truststore.password=${TRUSTSTORE_PASSWORD}
ssl.endpoint.identification.algorithm=
EOF

# 3-broker BCFKS cluster configs (ports 19092-19097)
for i in 1 2 3; do
  SSL_PORT=$((BCFKS_CLUSTER_BASE_PORT + (i-1) * 2))
  CTRL_PORT=$((BCFKS_CLUSTER_BASE_PORT + (i-1) * 2 + 1))

  cat > /tmp/quorum-bcfks-${i}.properties << EOF
node.id=${i}
process.roles=broker,controller
listeners=SSL://localhost:${SSL_PORT},CONTROLLER://localhost:${CTRL_PORT}
advertised.listeners=SSL://localhost:${SSL_PORT}
controller.listener.names=CONTROLLER
controller.quorum.voters=1@localhost:19093,2@localhost:19095,3@localhost:19097
inter.broker.listener.name=SSL
listener.security.protocol.map=SSL:SSL,CONTROLLER:SSL
log.dirs=/tmp/quorum-bcfks-${i}-logs
offsets.topic.replication.factor=3
transaction.state.log.replication.factor=3
transaction.state.log.min.isr=2
EOF
  cat /tmp/ssl-bcfks.conf >> /tmp/quorum-bcfks-${i}.properties
done

# 3-broker PEM cluster configs (ports 29092-29097)
for i in 1 2 3; do
  SSL_PORT=$((PEM_CLUSTER_BASE_PORT + (i-1) * 2))
  CTRL_PORT=$((PEM_CLUSTER_BASE_PORT + (i-1) * 2 + 1))

  cat > /tmp/quorum-pem-${i}.properties << EOF
node.id=${i}
process.roles=broker,controller
listeners=SSL://localhost:${SSL_PORT},CONTROLLER://localhost:${CTRL_PORT}
advertised.listeners=SSL://localhost:${SSL_PORT}
controller.listener.names=CONTROLLER
controller.quorum.voters=1@localhost:29093,2@localhost:29095,3@localhost:29097
inter.broker.listener.name=SSL
listener.security.protocol.map=SSL:SSL,CONTROLLER:SSL
log.dirs=/tmp/quorum-pem-${i}-logs
offsets.topic.replication.factor=3
transaction.state.log.replication.factor=3
transaction.state.log.min.isr=2
EOF
  cat /tmp/ssl-pem.conf >> /tmp/quorum-pem-${i}.properties
done
