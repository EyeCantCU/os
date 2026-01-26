#!/bin/bash
set -o errexit -o nounset -o errtrace -o pipefail -x

# Create directories for ClickHouse
mkdir -p clickhouse/tmp clickhouse/user_files clickhouse/format_schemas clickhouse/access

# Create minimal test configuration
cat > minimal_config.xml << EOF
<clickhouse>
    <http_port>8123</http_port>
    <tcp_port>9000</tcp_port>
    <openSSL>
        <client>
            <loadDefaultCAFile>true</loadDefaultCAFile>
            <cacheSessions>true</cacheSessions>
            <disableProtocols>sslv2,sslv3,tlsv1</disableProtocols>
            <preferServerCiphers>true</preferServerCiphers>
            <invalidCertificateHandler>
                <name>RejectCertificateHandler</name>
            </invalidCertificateHandler>
        </client>
    </openSSL>
    <watch_memory_usage_via_cgroups>false</watch_memory_usage_via_cgroups>
    <path>$(pwd)/clickhouse</path>
    <tmp_path>$(pwd)/clickhouse/tmp</tmp_path>
    <user_files_path>$(pwd)/clickhouse/user_files</user_files_path>
    <format_schema_path>$(pwd)/clickhouse/format_schemas</format_schema_path>
    <access_control_path>$(pwd)/clickhouse/access</access_control_path>
    <mark_cache_size>5368709120</mark_cache_size>
    <profiles>
        <default>
            <load_balancing>random</load_balancing>
        </default>
    </profiles>
    <users>
        <default>
            <password></password>
            <profile>default</profile>
            <quota>default</quota>
            <access_management>1</access_management>
        </default>
    </users>
    <quotas>
        <default>
            <interval>
                <duration>3600</duration>
                <queries>0</queries>
                <errors>0</errors>
                <result_rows>0</result_rows>
                <read_rows>0</read_rows>
                <execution_time>0</execution_time>
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

# Ensure all files and directories are accessible by the nonroot user
chown -R nonroot:nonroot /home/build
