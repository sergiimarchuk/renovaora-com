#!/bin/bash

set -a
source .env
set +a

echo "$DOCKERHUB_TOKEN" | docker login -u sergiimarchuk --password-stdin
docker pull sergiimarchuk/renovaora-com:latest
docker stop renovaora_web
docker rm renovaora_web
docker run -d --name renovaora_web -p 127.0.0.1:7878:8080 sergiimarchuk/renovaora-com:latest
