#!/bin/bash
# Test client-side FIPS cipher enforcement.
# Verifies clients refuse non-FIPS ciphers (ChaCha20, 3DES) and TLS 1.1.
# Tests against both running clusters (BCFKS and PEM).
set -o errexit -o nounset -o errtrace -o pipefail -x

export PATH="/usr/lib/kafka/bin:$PATH"

# Non-FIPS ciphers to test (cipher:protocol)
CIPHERS="TLS_CHACHA20_POLY1305_SHA256:TLSv1.3
TLS_RSA_WITH_3DES_EDE_CBC_SHA:TLSv1.2
TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA:TLSv1.2
TLS_DHE_RSA_WITH_3DES_EDE_CBC_SHA:TLSv1.2"

# Test against BCFKS cluster
for cipher_info in $CIPHERS; do
  cipher="${cipher_info%%:*}"
  protocol="${cipher_info##*:}"

  cp /tmp/client-bcfks.properties /tmp/bad-client.properties
  cat >> /tmp/bad-client.properties << EOF
ssl.enabled.protocols=${protocol}
ssl.cipher.suites=${cipher}
retries=0
request.timeout.ms=1000
default.api.timeout.ms=1000
EOF

  if kafka-topics.sh --list \
       --bootstrap-server localhost:${BCFKS_CLUSTER_BASE_PORT} \
       --command-config /tmp/bad-client.properties 2>&1 | tee /tmp/bad-client.log; then
    echo "ERROR: Client should have failed with ${cipher} (BCFKS)"
    exit 1
  fi
  grep -F "No usable cipher suites" /tmp/bad-client.log
done

# Test against PEM cluster
for cipher_info in $CIPHERS; do
  cipher="${cipher_info%%:*}"
  protocol="${cipher_info##*:}"

  cp /tmp/client-pem.properties /tmp/bad-client.properties
  cat >> /tmp/bad-client.properties << EOF
ssl.enabled.protocols=${protocol}
ssl.cipher.suites=${cipher}
retries=0
request.timeout.ms=1000
default.api.timeout.ms=1000
EOF

  if kafka-topics.sh --list \
       --bootstrap-server localhost:${PEM_CLUSTER_BASE_PORT} \
       --command-config /tmp/bad-client.properties 2>&1 | tee /tmp/bad-client.log; then
    echo "ERROR: Client should have failed with ${cipher} (PEM)"
    exit 1
  fi
  grep -F "No usable cipher suites" /tmp/bad-client.log
done

# TLS 1.1 should be rejected
for cluster in "BCFKS:${BCFKS_CLUSTER_BASE_PORT}:client-bcfks" "PEM:${PEM_CLUSTER_BASE_PORT}:client-pem"; do
  name="${cluster%%:*}"
  rest="${cluster#*:}"
  port="${rest%%:*}"
  config="${rest##*:}"

  cp /tmp/${config}.properties /tmp/bad-client.properties
  cat >> /tmp/bad-client.properties << EOF
ssl.enabled.protocols=TLSv1.1
retries=0
request.timeout.ms=1000
default.api.timeout.ms=1000
EOF

  if kafka-topics.sh --list \
       --bootstrap-server localhost:${port} \
       --command-config /tmp/bad-client.properties 2>&1 | tee /tmp/bad-client.log; then
    echo "ERROR: Client should have failed with TLS 1.1 (${name})"
    exit 1
  fi
  grep -E "(No usable cipher suites|SSLHandshakeException|protocol.*not supported|TLSv1.1)" /tmp/bad-client.log
done
