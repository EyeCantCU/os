#!/bin/bash

# Modified mongo setup test script from: https://github.com/rwynn/monstache/blob/rel6/docker/test/mongodb/scripts/mongo-engine-is-up.sh

set -eo pipefail;

mongoIsUp() {
  if [ -z "$1" ] || [ "$1" != "no-ssl" ] ; then
    if mongosh  --quiet "localhost:27017/test" --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 1)' > /dev/null 2>&1; then
      # echo 'Mongo-Ping: OK';
      return 0;
    else
      # echo 'Mongo-Ping: Not OK';
      return 1;
    fi
  else
    if mongosh --quiet "localhost:27017/test" --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 1)' > /dev/null 2>&1; then
      # echo 'Mongo-Ping: OK';
      return 0;
    else
      # echo 'Mongo-Ping: Not OK';
      return 1;
    fi
  fi
}

mongoIsUp "$1";

exit $?;
