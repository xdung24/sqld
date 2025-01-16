#!/bin/bash

VERSION=$(cat ./VERSION)
docker buildx build --progress=plain --platform linux/amd64 --push -t xdung24/sqld:$VERSION -t xdung24/sqld:latest .
