#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
ENV_FILE="${ENV_FILE:-$SCRIPT_DIR/.env}"
GO_IMAGE="${GO_IMAGE:-docker.m.daocloud.io/library/golang:1.26.2-alpine}"

read_env_value() {
  key="$1"
  [ -f "$ENV_FILE" ] || return 0
  awk -v key="$key" '
    $0 ~ "^[[:space:]]*" key "[[:space:]]*=" {
      sub(/^[^=]*=/, "")
      sub(/\r$/, "")
      print
      exit
    }
  ' "$ENV_FILE"
}

strip_matching_quotes() {
  value="$1"
  case "$value" in
    \"*\") value=${value#\"}; value=${value%\"} ;;
    \'*\') value=${value#\'}; value=${value%\'} ;;
  esac
  printf '%s' "$value"
}

env_or_file() {
  key="$1"
  current=$(eval "printf '%s' \"\${$key:-}\"")
  if [ -n "$current" ]; then
    printf '%s' "$current"
    return 0
  fi
  strip_matching_quotes "$(read_env_value "$key")"
}

SERVER_PORT="$(env_or_file SERVER_PORT)"
SERVER_PORT="${SERVER_PORT:-8080}"
E2E_BASE_URL="$(env_or_file E2E_BASE_URL)"
BASE_URL="$(env_or_file BASE_URL)"
BASE_URL="${BASE_URL:-${E2E_BASE_URL:-http://127.0.0.1:${SERVER_PORT}}}"
ONLINE_EXPERIENCE_USER_EMAIL="$(env_or_file ONLINE_EXPERIENCE_USER_EMAIL)"
ONLINE_EXPERIENCE_USER_PASSWORD="$(env_or_file ONLINE_EXPERIENCE_USER_PASSWORD)"
ONLINE_EXPERIENCE_GROUP_ID="$(env_or_file ONLINE_EXPERIENCE_GROUP_ID)"
ONLINE_EXPERIENCE_REQUIRE_UPSTREAM="$(env_or_file ONLINE_EXPERIENCE_REQUIRE_UPSTREAM)"
ONLINE_EXPERIENCE_REQUIRE_UPSTREAM="${ONLINE_EXPERIENCE_REQUIRE_UPSTREAM:-false}"

if [ -z "${ONLINE_EXPERIENCE_USER_EMAIL:-}" ] || [ -z "${ONLINE_EXPERIENCE_USER_PASSWORD:-}" ]; then
  printf '%s\n' "ONLINE_EXPERIENCE_USER_EMAIL and ONLINE_EXPERIENCE_USER_PASSWORD are required."
  printf '%s\n' "Set them in $ENV_FILE or export them before running this script."
  exit 2
fi

run_with_local_go() {
  (
    cd "$REPO_ROOT/backend"
    BASE_URL="$BASE_URL" \
    ONLINE_EXPERIENCE_USER_EMAIL="$ONLINE_EXPERIENCE_USER_EMAIL" \
    ONLINE_EXPERIENCE_USER_PASSWORD="$ONLINE_EXPERIENCE_USER_PASSWORD" \
    ONLINE_EXPERIENCE_GROUP_ID="${ONLINE_EXPERIENCE_GROUP_ID:-}" \
    ONLINE_EXPERIENCE_REQUIRE_UPSTREAM="$ONLINE_EXPERIENCE_REQUIRE_UPSTREAM" \
    go test -tags=e2e -v -run OnlineExperience ./internal/integration/...
  )
}

docker_base_url="$BASE_URL"
case "$docker_base_url" in
  http://127.0.0.1:*) docker_base_url="http://host.docker.internal:${docker_base_url#http://127.0.0.1:}" ;;
  http://localhost:*) docker_base_url="http://host.docker.internal:${docker_base_url#http://localhost:}" ;;
esac

run_with_docker_go() {
  docker run --rm \
    --add-host=host.docker.internal:host-gateway \
    -e BASE_URL="$docker_base_url" \
    -e ONLINE_EXPERIENCE_USER_EMAIL="$ONLINE_EXPERIENCE_USER_EMAIL" \
    -e ONLINE_EXPERIENCE_USER_PASSWORD="$ONLINE_EXPERIENCE_USER_PASSWORD" \
    -e ONLINE_EXPERIENCE_GROUP_ID="${ONLINE_EXPERIENCE_GROUP_ID:-}" \
    -e ONLINE_EXPERIENCE_REQUIRE_UPSTREAM="$ONLINE_EXPERIENCE_REQUIRE_UPSTREAM" \
    -v "$REPO_ROOT/backend:/workspace" \
    -w /workspace \
    "$GO_IMAGE" \
    go test -tags=e2e -v -run OnlineExperience ./internal/integration/...
}

printf 'Running online experience E2E against %s\n' "$BASE_URL"

if command -v go >/dev/null 2>&1; then
  run_with_local_go
else
  run_with_docker_go
fi
