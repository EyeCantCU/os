#!/usr/bin/env bash

set -o errexit -o nounset -o errtrace -o pipefail -x

JAVA_HOME="${DESTDIR}/${JAVA_HOME}"
CLASSPATH="/usr/share/java/bouncycastle-fips/*:."
export CLASSPATH

"${JAVA_HOME}/bin/javac" Test.java
"${JAVA_HOME}/bin/java" Test
