#!/bin/bash
set -xeou pipefail

DOCKER_REGISTRY=${DOCKER_REGISTRY:-kubedb}

IMG=mongo

DB_VERSION=4.1
PATCH=4.1.13

TAG="$DB_VERSION"
BASE_TAG="$PATCH"

docker pull "$DOCKER_REGISTRY/$IMG:$BASE_TAG"

docker tag "$DOCKER_REGISTRY/$IMG:$BASE_TAG" "$DOCKER_REGISTRY/$IMG:$TAG"
docker push "$DOCKER_REGISTRY/$IMG:$TAG"
