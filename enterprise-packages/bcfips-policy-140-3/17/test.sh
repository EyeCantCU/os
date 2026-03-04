#!/usr/bin/env bash

set -o errexit -o nounset -o errtrace -o pipefail -x

CLASSPATH="${CLASSPATH:-/usr/share/java/bouncycastle-fips/*:.}"
export CLASSPATH

SECURITY="-Djava.security.manager=default -Djava.security.debug=access,failure"
if [ "$JAVA_VERSION" -gt "23" ]; then
    SECURITY=""
fi

"${JAVA_HOME}/bin/javac" Test.java
timeout 120 "${JAVA_HOME}/bin/java" $SECURITY Test
"${JAVA_HOME}/bin/java" $SECURITY org.bouncycastle.entropy.util.DumpInfo
