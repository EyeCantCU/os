#!/bin/bash

# Modified mongo setup test script from: https://github.com/rwynn/monstache/blob/rel6/docker/test/mongodb/scripts/mongo-users-setup.sh

echo "************************************************************"
echo "Begin setting up users..."
echo "************************************************************"

mongodb_setup_cmd="mongod"

mongodb_setup_cmd="$mongodb_setup_cmd --dbpath /var/lib/mongo --logpath /var/log/mongodb/mongod.log --fork"

echo "Starting the server";
$mongodb_setup_cmd

echo "Waiting for engine to start";
./mongo-engine-wait.sh no-ssl

# create root user
if [ ! -z "${MONGO_USER_ROOT_NAME+x}" ] && [ ! -z "${MONGO_USER_ROOT_PASSWORD+x}" ] ; then
  mongosh --eval "db.createUser({user: '$MONGO_USER_ROOT_NAME', pwd: '$MONGO_USER_ROOT_PASSWORD', roles:[{ role: 'dbAdmin', db: 'test' } ]});"
else
  echo 'ERROR: Mongo root user credentials are not provided!';
  exit 1;
fi

echo "Shutting down...";
mongosh --eval "db.shutdownServer();";

echo 'Sleeping 1 second...';
sleep 1;

echo "************************************************************"
echo "End Setting up users..."
echo "************************************************************"
