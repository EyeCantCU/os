#!/usr/bin/env bash

set -o errexit -o nounset -o errtrace -o pipefail -x
set -eu

cp java.security "${DESTDIR}/usr/lib/jvm/java-${VERSION}-openjdk/conf/security"

mkdir -p "${DESTDIR}/usr/lib/jvm/jdk-fips-config"
cp java.policy "${DESTDIR}/usr/lib/jvm/jdk-fips-config"
