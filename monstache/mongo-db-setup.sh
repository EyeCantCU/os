#!/bin/bash

# From: https://github.com/rwynn/monstache/tree/rel6/docker/test/mongodb/scripts

echo "************************************************************"
echo "Setting up database"
echo "************************************************************"

set -eo pipefail;

./mongo-engine-wait.sh

./mongo-rep-set-wait.sh
