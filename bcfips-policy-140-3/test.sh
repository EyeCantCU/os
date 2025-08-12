#!/usr/bin/env bash

set -o errexit -o nounset -o errtrace -o pipefail -x

CLASSPATH="${CLASSPATH:-/usr/share/java/bouncycastle-fips/*:.}"
export CLASSPATH

"${JAVA_HOME}/bin/javac" Test.java
timeout 120 "${JAVA_HOME}/bin/java" Test
"${JAVA_HOME}/bin/java" org.bouncycastle.entropy.util.DumpInfo
