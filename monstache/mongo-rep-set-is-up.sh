#!/bin/bash

# From: https://github.com/rwynn/monstache/tree/rel6/docker/test/mongodb/scripts

set -eo pipefail;

replicaSetIsUp() {
  # If ReplicaSet is initialized and the current instance (mongo-0) is primary (myState:1)
  if mongosh -u "$MONGO_USER_ROOT_NAME" -p "$MONGO_USER_ROOT_PASSWORD" --authenticationDatabase "test" --quiet --eval "var status = rs.status(); status.ok === 1 && status.myState === 1 ? quit(0) : quit(1)" > /dev/null 2>&1 ; then
    # echo 'ReplicaSet-Primary: Status is OK and and I am primary ..';
    return 0;
  else
    # echo 'ReplicaSet-Primary: Not yet ..';
    return 1;
  fi
}

replicaSetIsUp;

exit $?;
