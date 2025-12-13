#!/bin/bash

set -e
set -u
set -x
#############
# Environment

workspace=$(dirname $0)
if ! echo $workspace |  grep "^/" ;  then
  workspace="$(pwd)/$workspace"
fi
repository='/tmp'
changedFiles="$(git diff --name-only HEAD^1 HEAD)"

if [ $(uname -m) = "x86_64" ] ; then
  arch="x86_64"
else
  arch="armv7hf"
fi

################
# Login creation

if [ ! -f ~/.docker/config.json ] ; then 
  mkdir -p ~/.docker/

  if [ -z "${DOCKER_LOGIN:-}" ] ; then
    echo "Docker login not found in the environment, set DOCKER_LOGIN"
  else
    cat << EOF > ~/.docker/config.json
{
  "experimental": "enabled",
        "auths": {
                "https://index.docker.io/v1/": {
                        "auth": "$DOCKER_LOGIN"
                }
        },
        "HttpHeaders": {
                "User-Agent": "Docker-Client/17.12.1-ce (linux)"
        }
}
EOF
  fi
fi

#######
# Build

docker build -t fabrizio2210/syncplayed:${arch} -f docker/x86_64/Dockerfile-syncplayed .

######
# Push

docker push fabrizio2210/syncplayed:${arch}

#######
# Clean
docker container prune --force || true
docker image prune --force || true
