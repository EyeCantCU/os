#@test
#@summary
# Verifies that required license and related Zulu files exist in the Zulu directory.

FS_BUF=$FS
FS=" "

list="readme.txt DISCLAIMER Welcome.html"	

failed=false
cd $TESTJAVA
LICENCE_FILES=`find . -name "LICENSE"`

count=$(echo "$LICENCE_FILES" | wc -l)
if [ "$count" -lt 1 ]; then
    echo "FAIL: No license file found in the binary"
    failed=true
fi

echo "Build is CPE build, try to find CPEN files"
CPEN_FILES=""
for file in $LICENCE_FILES; do
    CPEN_FILES="$CPEN_FILES ${file/LICENSE/CLASSPATH_EXCEPTION_NOTE}"
done
list="$CPEN_FILES $list"

echo "Try to find $list in $TESTJAVA:"
for FILENAME in $list
do
  FILE="$TESTJAVA/$FILENAME"
  if [ ! -f "$FILE" ]; then
    failed=true
    echo "FAIL: File $FILENAME does not exist in $TESTJAVA"
  else
    echo "File $FILENAME exists in $TESTJAVA"
  fi
done
FS=$FS_BUF
if [ "$failed" = true ] ; then
  echo "FAIL: Some files do not exist for installation"
  exit 1
elif [ "$failed" = false ] ; then
  echo "PASS: All files exist as expected"
fi
