#!/bin/bash

VERSION=$(cat ./VERSION)
docker buildx create --use --name mybuilder
docker buildx build --platform linux/amd64 -t xdung24/sqld:$VERSION -t xdung24/sqld:latest --push .
