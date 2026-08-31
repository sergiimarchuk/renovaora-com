#!/bin/bash
# cleanup_containers.sh - stop running containers and remove exited ones

echo "=== Stopping all running containers ==="
docker ps -q | xargs -r docker stop

echo "=== Removing all exited/stopped containers ==="
docker ps -a -q -f status=exited | xargs -r docker rm

echo "=== Done. Current container list: ==="
docker ps -a
