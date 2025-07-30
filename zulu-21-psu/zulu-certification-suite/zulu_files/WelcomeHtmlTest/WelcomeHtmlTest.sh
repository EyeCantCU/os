#@test
#@summary
# Checks Welcome.html presence and that its copyright line correctly spans
# from the first release year to the current year, verifying content against a template.

set -x
case "$MAJOR_VERSION" in
  25 ) FIRST_RELEASE_YEAR=2025 ;;
  24 ) FIRST_RELEASE_YEAR=2024 ;;
  21 ) FIRST_RELEASE_YEAR=2022 ;;
esac

CURRENT_YEAR=$( date +"%Y" )
month=$( date +"%m" )
if [ "$month" = "12" ]; then
   CURRENT_YEAR=`expr $CURRENT_YEAR + 1`
   echo "CURRENT_YEAR set to $CURRENT_YEAR because we test Yan release in Dec"
fi

if [ -z "$TESTJAVA" ] ; then
  echo "Error: TESTJAVA is not set"
  exit 1
fi

[ -d "$TESTSRC" ] || TESTSRC=`dirname $0`

WELCOME_HTML="$TESTJAVA/Welcome.html"

if [ ! -e "$WELCOME_HTML" ] ; then
  echo "Error: Welcome.html doesn't exist at $TESTJAVA"
  ls "$TESTJAVA"
  exit 1
fi

WELCOME_HTML_TEMPLATE="$TESTSRC/WelcomeHtml.template"
WELCOME_HTML_GENERATED=Welcome.html.generated
WELCOME_HTML_FILTERED=Welcome.html.filtered

YEARS_LINE=$( cat "$WELCOME_HTML" | grep "$FIRST_RELEASE_YEAR" | grep "Copyright" )

if [ `echo "$YEARS_LINE" | grep "$CURRENT_YEAR" | wc -l`  -eq 0 ] ; then
  echo "Error: welcome.html should be updated with a current year: $CURRENT_YEAR, copyright line: $YEARS_LINE"
  exit 1
fi

# EA, we start e.g. in 2024 and going to release it next year
if [ $FIRST_RELEASE_YEAR -gt $CURRENT_YEAR ] ; then
  YEARS=$( echo $YEARS_LINE | sed "s/.* \($CURRENT_YEAR.*$FIRST_RELEASE_YEAR\).*/\1/" )
# usual release, we start e.g. in 2014 and still support it
elif [ $FIRST_RELEASE_YEAR -lt $CURRENT_YEAR ] ; then
  YEARS=$( echo $YEARS_LINE | sed "s/.* \($FIRST_RELEASE_YEAR.*$CURRENT_YEAR\).*/\1/" )
else
  YEARS="$CURRENT_YEAR"
fi

echo "YEARS: $YEARS"

cat "$WELCOME_HTML_TEMPLATE" | sed "s#%YEARS%#${YEARS}#g" > "$WELCOME_HTML_GENERATED"
cat "$WELCOME_HTML" | sed '/^ *$/d;s/ *$//' > "$WELCOME_HTML_FILTERED"

diff "$WELCOME_HTML_FILTERED" "$WELCOME_HTML_GENERATED"

exit $?
