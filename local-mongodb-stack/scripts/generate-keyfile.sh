#!/bin/zsh

mkdir -p ./secrets
openssl rand -base64 756 > ./secrets/mongo-keyfile
chmod 400 ./secrets/mongo-keyfile