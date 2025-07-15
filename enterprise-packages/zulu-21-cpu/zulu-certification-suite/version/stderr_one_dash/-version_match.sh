#@test
#@summary
# Verifies that 'java -version' output has exactly three lines
# and that each line matches the expected OpenJDK and Zulu version patterns.
# https://openjdk.org/jeps/322

echo "$TESTJAVA/bin/java$exe -version"
$TESTJAVA/bin/java$exe -version > version.out 2>&1

cat version.out

java_version="$JAVA_VERSION"
zulu_version="$ZULU_VERSION"

status=0

if [ `cat version.out | wc -l` -ne 3 ] ; then
  echo 'FAIL java -version output should have three lines'
  status=`expr $status + 2`
fi

reg1="^openjdk version \"$java_version\""
if [ `cat version.out | sed -n 1p | grep "$reg1" | wc -l` -ne 1 ] ; then
  echo "First line is not matched by '$reg1'"
  status=`expr $status + 4`
fi

reg2="^OpenJDK Runtime Environment Zulu.*$zulu_version-$BRANDING.*build $java_version"
if [ `cat version.out | sed -n 2p | grep -i "$reg2" | wc -l` -ne 1 ] ; then
  echo "Second line is not matched by '$reg2'"
  status=`expr $status + 8`
fi

reg3="^OpenJDK .* VM Zulu.*$zulu_version-$BRANDING.*build"
if [ `cat version.out | sed -n 3p | grep -i "$reg3" | wc -l` -ne 1 ] ; then
  echo "Third line is not matched by '$reg3'"
  status=`expr $status + 16`
fi

exit $status
