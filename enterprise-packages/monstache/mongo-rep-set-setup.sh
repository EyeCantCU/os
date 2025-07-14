#!/bin/bash

# From: https://github.com/rwynn/monstache/blob/rel6/docker/test/mongodb/scripts/mongo-rep-set-setup.sh

echo "************************************************************"
echo " Begin setting up replica set"
echo "************************************************************"


./mongo-engine-wait.sh

# Login as root and configure replica set
# # https://docs.mongodb.com/manual/reference/method/rs.initiate/#rs.initiate
mongosh -u "$MONGO_USER_ROOT_NAME" -p "$MONGO_USER_ROOT_PASSWORD" --authenticationDatabase "test" --eval "rs.initiate({ '_id': '$MONGO_REPLICA_SET_NAME', 'version': 1, 'members': $MONGO_REPLICA_SET_MEMBERS });"

echo "************************************************************"
echo " End setting up replica set"
echo "************************************************************"
