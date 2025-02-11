#!/bin/bash

# Modified mongo setup test script from: https://github.com/rwynn/monstache/blob/rel6/docker/test/mongodb/scripts/mongo-run.sh
set -m -eoux pipefail;

# Setup data and log dirs
mkdir -p /var/lib/mongo
mkdir -p /var/log/mongodb
chown $(whoami) /var/lib/mongo
chown $(whoami) /var/log/mongodb

# Setup mongo user
./mongo-users-setup.sh;

# Setup mongdb cmd and options
mongodb_cmd="mongod"
cmd="$mongodb_cmd"
cmd="$cmd --storageEngine wiredTiger"
cmd="$cmd --oplogSize 128"
cmd="$cmd --bind_ip 127.0.0.1"

if [[ "$MONGO_REPLICA_SET_NAME" ]] ; then
  cmd="$cmd --replSet $MONGO_REPLICA_SET_NAME"
fi

cmd="$cmd --dbpath /var/lib/mongo --logpath /var/log/mongodb/mongod.log --fork"

# Start mongdb server
$cmd

# if [ "$MONGO_ROLE" == "primary" ]; then
./mongo-rep-set-setup.sh
# fi

./mongo-db-setup.sh

# Quick health check
echo "************************************************************"
echo "Performing health check"
echo "************************************************************"
mongosh "mongodb://localhost:27017" --eval "printjson(db.runCommand({hello: 1}))"
