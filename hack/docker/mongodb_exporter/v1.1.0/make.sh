#!/bin/bash
set -eou pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT=$GOPATH/src/kubedb.dev/mongodb

source "$REPO_ROOT/hack/libbuild/common/lib.sh"
source "$REPO_ROOT/hack/libbuild/common/kubedb_image.sh"

DOCKER_REGISTRY=${DOCKER_REGISTRY:-kubedb}
IMG=mongodb_exporter
TAG=v1.1.0

build() {
  pushd "$REPO_ROOT/hack/docker/mongodb_exporter/$TAG"

  # Download mongodb_exporter. github repo: https://github.com/percona/mongodb_exporter
  # Prometheus Exporters link: https://prometheus.io/docs/instrumenting/exporters/
  wget -O mongodb_exporter.tar.gz https://github.com/percona/mongodb_exporter/releases/download/v0.8.0/mongodb_exporter-0.8.0.linux-amd64.tar.gz
  tar xf mongodb_exporter.tar.gz
  chmod +x mongodb_exporter

  local cmd="docker build --pull -t $DOCKER_REGISTRY/$IMG:$TAG ."
  echo $cmd; $cmd

  rm mongodb_exporter mongodb_exporter.tar.gz
  popd
}

binary_repo $@
