#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-.local/exit-server.env}"

if [ ! -f "$ENV_FILE" ]; then
    echo "Missing environment file: $ENV_FILE" >&2
    echo "Create it outside git with EXIT_SERVER_HOST, EXIT_SERVER_USER, EXIT_SERVER_PORT, and EXIT_SERVER_KEY_FILE." >&2
    exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"

: "${EXIT_SERVER_HOST:?EXIT_SERVER_HOST is required}"
: "${EXIT_SERVER_USER:?EXIT_SERVER_USER is required}"
: "${EXIT_SERVER_PORT:?EXIT_SERVER_PORT is required}"
: "${EXIT_SERVER_KEY_FILE:?EXIT_SERVER_KEY_FILE is required}"

ssh \
    -i "$EXIT_SERVER_KEY_FILE" \
    -p "$EXIT_SERVER_PORT" \
    -o StrictHostKeyChecking=no \
    "$EXIT_SERVER_USER@$EXIT_SERVER_HOST" \
    'uname -a; arch; hostname; id'
