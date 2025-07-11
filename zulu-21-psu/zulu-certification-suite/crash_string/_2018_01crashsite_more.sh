#@test
#@summary
# Verifies that when the JVM aborts on a RuntimeException using
# -XX:AbortVMOnException, the crash output contains the expected
# vendor support URL (http://www.azul.com/support/).
#@requires ( jdk.version.major >= 9 )
#@build ThrowRuntimeException
#@run shell _2018_01crashsite_more.sh

"$TESTJAVA/bin/java$exe" -cp $TESTCLASSES -XX:+UnlockDiagnosticVMOptions -XX:AbortVMOnException=java.lang.RuntimeException ThrowRuntimeException | tee crash.txt

AZULSTRING="http://www.azul.com/support/"

echo "Verifying crash test contains $AZULSTRING..."

grep "$AZULSTRING" crash.txt

exit $?
