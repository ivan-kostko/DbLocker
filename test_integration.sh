#!/bin/bash
#
# This scripts runs all the  integration tests. It uses Docker Compose to
# set up an environment. It is not supposed to run in a Docker container.
#

echo "Stopping and removing running containers"
docker-compose stop
docker-compose rm -f
echo "Running integration tests..."
docker-compose run --rm -T test_integration go test -v -tags integration -race ./...
