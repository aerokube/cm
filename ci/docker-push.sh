#!/usr/bin/env bash

set -e

docker login -u="$DOCKER_USERNAME" -p="$DOCKER_PASSWORD"
docker build --pull -t "$GITHUB_REPOSITORY" -t "$GITHUB_REPOSITORY:$1" .
docker push "$GITHUB_REPOSITORY"
docker push "$GITHUB_REPOSITORY:$1"
