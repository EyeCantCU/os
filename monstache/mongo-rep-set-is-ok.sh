#!/bin/bash

# From: https://github.com/rwynn/monstache/tree/rel6/docker/test/mongodb/scripts

set -eo pipefail;

replicaSetStatusIsOk() {
  if mongosh -u "$MONGO_USER_ROOT_NAME" -p "$MONGO_USER_ROOT_PASSWORD" --authenticationDatabase "test" --quiet --eval 'quit(rs.status().ok ? 0 : 1)' > /dev/null 2>&1 ; then
    # echo 'ReplicaSet-Status: OK';
    return 0;
  else
    # echo 'ReplicaSet-Status: Not OK';
    return 1;
  fi
}

replicaSetStatusIsOk;

exit $?;
