#@test
#@summary
# Verifies the contents of readme.txt against expected text, ensuring Zulu version.

[ -d "$TESTSRC" ] || TESTSRC=`dirname $0`

README="$TESTJAVA/readme.txt"
README_GOLDEN_PATTERN="$TESTSRC/readme.pattern.golden"

if [ ! -f "$README" ]; then
  echo "$README not found!"
  exit 1
fi

echo "cat $README"
cat "$README"

echo

status=0

echo "cat '$README_GOLDEN_PATTERN' | sed 's/\\\$ZULU_VERSION/$ZULU_VERSION/' >readme.golden"
cat "$README_GOLDEN_PATTERN" | sed "s/\\\$ZULU_VERSION/$ZULU_VERSION/" >readme.golden

echo

echo "diff readme.golden '$README'"
diff readme.golden $README || status=3

echo "status=$status"

exit $status