#!/bin/bash
# 1. Verify inter-broker (controller) ports reject non-FIPS TLS 1.2 cipher.
# 2. Verify inter-broker (controller) ports accept valid TLS 1.2 cipher.
set -o errexit -o nounset -o errtrace -o pipefail -x

# Controller ports: 19093, 19095, 19097 (BCFKS) and 29093, 29095, 29097 (PEM)
BCFKS_CTRL_PORTS="19093 19095 19097"
PEM_CTRL_PORTS="29093 29095 29097"

# Disable pipefail (openssl returns non-zero on rejected connection)
set +o pipefail

# Non-FIPS cipher (ChaCha20) should be rejected - expect SSL alert 40 (handshake_failure)
for port in $BCFKS_CTRL_PORTS $PEM_CTRL_PORTS; do
  openssl s_client -connect localhost:${port} \
    -tls1_2 \
    -cipher 'ECDHE-RSA-CHACHA20-POLY1305' \
    -CAfile /tmp/kafka.crt </dev/null 2>&1 | grep -E 'error:0A000410:SSL routines.*SSL alert number 40'
done

set -o pipefail

# FIPS cipher (AES-GCM) with TLS 1.2 should be accepted
for port in $BCFKS_CTRL_PORTS $PEM_CTRL_PORTS; do
  openssl s_client -connect localhost:${port} \
    -tls1_2 \
    -cipher 'ECDHE-RSA-AES256-GCM-SHA384' \
    -CAfile /tmp/kafka.crt </dev/null 2>&1 | grep -F "Cipher is ECDHE-RSA-AES256-GCM-SHA384"
done
