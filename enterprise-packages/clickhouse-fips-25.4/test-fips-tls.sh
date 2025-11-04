#!/bin/bash
set -o errexit -o nounset -o errtrace -o pipefail -x

# Verify server is running with HTTP
curl -s "http://localhost:8123/?query=SELECT%201" | grep -q "1"

# Test 1: Regular FIPS-compliant TLS connection
RESULT=$(openssl s_client -connect localhost:8443 -servername localhost -brief 2>&1)

# Check for TLSv1.2 or TLSv1.3 (FIPS-compliant)
# Use basic grep with extended regex to be more explicit
echo "$RESULT" | grep -qE "TLSv1\.(2|3)" || {
  echo "ERROR: TLS connection not using FIPS-compliant protocol"
  echo "$RESULT" | grep -i "Protocol"
  exit 1
}

# Check that we're NOT using weak ciphers
echo "$RESULT" | grep -qiE "Ciphersuite.*(RC4|DES|MD5)" && {
  echo "ERROR: TLS connection using non-FIPS compliant cipher"
  echo "$RESULT" | grep -i "Ciphersuite"
  exit 1
}

# Check that we ARE using FIPS-approved ciphers (AES, etc.)
echo "$RESULT" | grep -qi "Ciphersuite.*AES" || {
  echo "ERROR: TLS connection not using FIPS-compliant cipher"
  echo "$RESULT" | grep -i "Ciphersuite"
  exit 1
}

# Test 2: Try non-FIPS protocol (should fail - error expected)
curl -k -s -v --tlsv1.1 --tls-max 1.1 https://localhost:8443?query=SELECT%201 && {
  echo "Protocol test failed: Server accepted non-FIPS compliant protocol"
  exit 1
} || {
  # Need to wait for server logs to show the bad connection attempt
  sleep 2
  cat clickhouse-server.log | grep "SSL Exception" | grep "routines::unexpected message"
}

# This errors on the client side in a FIPS environment - may need to test weak certs in the image tests
# # Test 3: Try to connect with weak certs (should fail - error expected)
# curl -k -s -v --cert clickhouse-certs/weak-cert.pem --key clickhouse-certs/weak-key.pem https://localhost:8443?query=SELECT%201 && {
#   echo "Certificate test failed: Server accepted connection with weak (1024-bit) certificate"
#   exit 1
# } || {
#   sleep 5
#   cat clickhouse-server.log
# }
