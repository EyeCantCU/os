#@test
#@summary
# Verifies that the output of java.version matches JAVA_VERSION or MAJOR_VERSION pattern.
#@build JavaVersionTest
#@run shell java_version.sh

set -x

echo "$TESTJAVA/bin/java$exe -cp $TESTCLASSES JavaVersionTest"
$TESTJAVA/bin/java$exe -cp $TESTCLASSES JavaVersionTest
$TESTJAVA/bin/java$exe -cp $TESTCLASSES JavaVersionTest > java_version.actual

status=0

if [ -n "$JAVA_VERSION" ] ; then
  version=$JAVA_VERSION
  cmd=diff
else
  version="$MAJOR_VERSION[0-9.]*"
  cmd=grep
fi

echo "Expected: $version"

if [ "$cmd" = diff ] ; then
  echo "$version" >  java_version.expected
  diff -w java_version.expected java_version.actual || status=3
elif [ "$cmd" = grep ] ; then
  cat java_version.actual | grep "^$version$" || status=5
else
  echo Test error
  status=7
fi

exit $status