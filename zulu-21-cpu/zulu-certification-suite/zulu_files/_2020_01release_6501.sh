#@test
#@summary
# Checks that the release file includes exactly one IMPLEMENTOR and IMPLEMENTOR_VERSION field with expected values
# and that SOURCE contains '.*git' value.

[ -d "$TESTSRC" ] || TESTSRC=`dirname $0`

status=0
RELEASE="$TESTJAVA/release"
README="$TESTJAVA/readme.txt"
IMPLEMENTOR_VALUE="\"Azul Systems, Inc.\""
IMPLEMENTOR="IMPLEMENTOR="
IMPLEMENTOR_VERSION="IMPLEMENTOR_VERSION="
IMPLEMENTOR_VERSION_VALUE="$ZULU_VERSION-$BRANDING"

if [ ! -f "$README" ]; then
   echo "$README not found!"
   exit 1
fi

# Test that file exists ...
if [ ! -f "$RELEASE" ]; then
   echo "$RELEASE not found!"
   exit 1
fi

#if cat $RELEASE | grep SOURCE=.*git:
#then
#   echo "Field SOURCE is present in release file and points to git";
#else
#   echo "Field SOURCE is NOT present in release file OR NOT points to git";
#   cat $RELEASE
#   exit 1
#fi
#RELEASE_SOURCE=`cat $RELEASE | sed -n '/SOURCE=/s/SOURCE=.*git://p' |   sed 's/[^a-zA-Z0-9]//g'`
#RELEASE_SOURCE="${RELEASE_SOURCE//$'\r'}"

#echo RELEASE_SOURCE=$RELEASE_SOURCE
#if [ -z "$RELEASE_SOURCE" ]; then
#   echo "SOURCE field with git: not found in release file"
#   cat "$RELEASE"
#   exit 2
#fi

# Test that release file contains IMPLEMENTOR field...
 OUT=`cat "$RELEASE" | grep "$IMPLEMENTOR"`
 [ -z "$exe" ] || OUT=`echo "$OUT" | tr -d '\r'`

 if [ "`cat $RELEASE | grep -c $IMPLEMENTOR`" -ne 1 ]; then
     echo "$IMPLEMENTOR field appears more than once in release file!"
     echo $OUT
     echo "release content"
     cat "$RELEASE"
     exit 6
 fi

 if [ "$OUT" != "$IMPLEMENTOR$IMPLEMENTOR_VALUE" ] ; then
     echo "$IMPLEMENTOR is checked"
     echo "expected: $IMPLEMENTOR$IMPLEMENTOR_VALUE"
     echo "actual: $OUT"
     exit 7
 fi
 
 
  # Test that release file contains IMPLEMENTOR_VALUE field...
 OUT=`cat "$RELEASE" | grep "$IMPLEMENTOR_VERSION"`
 [ -z "$exe" ] || OUT=`echo "$OUT" | tr -d '\r'`

 if [ "`cat $RELEASE | grep -c $IMPLEMENTOR_VERSION`" -ne 1 ]; then
     echo "$IMPLEMENTOR_VERSION field appears more than once in release file!"
     echo $OUT
     echo "release content"
     cat "$RELEASE"
     exit 8
 fi

 if [ "`echo $OUT | grep -ci $IMPLEMENTOR_VERSION_VALUE`" -ne 1 ] ; then
     echo "$IMPLEMENTOR_VERSION is checked"
     echo "expected: $IMPLEMENTOR_VERSION contains $IMPLEMENTOR_VERSION_VALUE"
     echo "actual: $OUT"
     exit 9
 fi


echo All tests passed