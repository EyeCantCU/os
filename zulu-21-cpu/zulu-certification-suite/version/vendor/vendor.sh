#@test
#@summary
# Verifies JVM vendor properties from -XshowSettings:properties
# match the expected vendor.pattern, including correct Zulu version substitution.

set -x
set -e

[ -d "$TESTSRC" ] || TESTSRC=`dirname $0`

echo "$TESTJAVA/bin/java$exe -XshowSettings:properties"
$TESTJAVA/bin/java$exe -XshowSettings:properties > properties.out 2>&1 || true

$TESTJAVA/bin/java$exe -version > version.out 2>&1

cat properties.out | grep '\.vendor' | sort > vendor.out
cat vendor.out

cat version.out | sed -n 2p | \
  sed 's/^OpenJDK Runtime Environment[ (]*//;s/[ ()]*build.*//' > zulu.out


zulu=`cat zulu.out`
echo Zulu version: $zulu
cat "$TESTSRC/vendor.pattern" | grep -v jdk.vendor.version | sort > vendor.pattern

cat vendor.pattern | sed "s/%%ZULU_VERSION%%/$zulu/" > vendor.golden

echo Golden:
cat vendor.golden

diff -w vendor.golden vendor.out
exit $?
