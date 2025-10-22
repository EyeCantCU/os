#!/bin/bash
set -o errexit -o nounset -o errtrace -o pipefail -x

# Create directories for ClickHouse
mkdir -p clickhouse-certs clickhouse/tmp clickhouse/user_files clickhouse/format_schemas

# Create minimal test configuration with TLS support
cat > test_config.xml << EOF
<clickhouse>
    <http_port>8123</http_port>
    <https_port>8443</https_port>
    <openSSL>
        <server>
            <certificateFile>$(pwd)/clickhouse-certs/clickhouse-cert.pem</certificateFile>
            <privateKeyFile>$(pwd)/clickhouse-certs/clickhouse-key.pem</privateKeyFile>
            <verificationMode>none</verificationMode>
            <invalidCertificateHandler>
                <n>AcceptCertificateHandler</n>
            </invalidCertificateHandler>
            <protocols>TLSv1.2,TLSv1.3</protocols>
            <preferServerCiphers>true</preferServerCiphers>
        </server>
    </openSSL>
    <watch_memory_usage_via_cgroups>false</watch_memory_usage_via_cgroups>
    <path>$(pwd)/clickhouse</path>
    <tmp_path>$(pwd)/clickhouse/tmp</tmp_path>
    <user_files_path>$(pwd)/clickhouse/user_files</user_files_path>
    <format_schema_path>$(pwd)/clickhouse/format_schemas</format_schema_path>
    <status_file_path>$(pwd)/clickhouse/status</status_file_path>
    <profiles>
        <default></default>
    </profiles>
    <users>
        <default>
            <password></password>
            <profile>default</profile>
            <quota>default</quota>
        </default>
    </users>
    <quotas>
        <default>
            <interval>
                <duration>3600</duration>
                <queries>0</queries>
            </interval>
        </default>
    </quotas>
    <logger>
        <level>debug</level>
        <console>1</console>
        <log>$(pwd)/clickhouse-server.log</log>
        <errorlog>$(pwd)/clickhouse-server.err.log</errorlog>
    </logger>
</clickhouse>
EOF

# Create standard FIPS-compliant certificate
openssl req -new -newkey rsa:2048 -days 1 -nodes -x509 \
  -subj "/C=US/ST=Test/L=Test/O=Test/CN=localhost" \
  -keyout clickhouse-certs/clickhouse-key.pem -out clickhouse-certs/clickhouse-cert.pem

# Temporarily disable FIPS by moving the config file and creating a basic replacement
if [ -f /etc/ssl/openssl.cnf ]; then
  # Save FIPS config
  mv /etc/ssl/openssl.cnf /etc/ssl/openssl.cnf.fips
  
  # Create a minimal non-FIPS OpenSSL config
  cat > /etc/ssl/openssl.cnf << EOF
[ req ]
default_bits           = 1024
default_keyfile        = privkey.pem
distinguished_name     = req_distinguished_name
attributes             = req_attributes
prompt                 = no

[ req_distinguished_name ]
C                      = US
ST                     = Test
L                      = Test
O                      = Test
OU                     = Test
CN                     = weak
emailAddress           = test@example.com

[ req_attributes ]
challengePassword      = test
EOF
  
  # Create weak certificate (1024-bit, non-FIPS compliant)
  openssl req -new -newkey rsa:1024 -days 1 -nodes -x509 \
    -subj "/C=US/ST=Test/L=Test/O=Test/CN=weak" \
    -keyout clickhouse-certs/weak-key.pem -out clickhouse-certs/weak-cert.pem
  
  # Restore FIPS config so server starts in FIPS mode
  mv /etc/ssl/openssl.cnf.fips /etc/ssl/openssl.cnf
fi

chown -R nonroot:nonroot /home/build
