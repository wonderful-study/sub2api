#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_PATH="${SCRIPT_DIR}/$(basename "${BASH_SOURCE[0]}")"
ROOT_DIR=""
DEPLOY_DIR=""
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
OUTPUT_PATH=""
COMPOSE_FILE=""
PACKAGE_COMPOSE_SOURCE=""

STOP_STACK=false
RESTART_STACK=false
MANUAL_COMPOSE_FILE=""

for arg in "$@"; do
  case "$arg" in
    --stop-stack)
      STOP_STACK=true
      ;;
    --restart-stack)
      RESTART_STACK=true
      ;;
    --compose-file=*)
      MANUAL_COMPOSE_FILE="${arg#*=}"
      ;;
    *.tar.gz)
      OUTPUT_PATH="$arg"
      ;;
  esac
done

resolve_layout() {
  if [[ -f "${SCRIPT_DIR}/.env" || -f "${SCRIPT_DIR}/docker-compose.yml" || -f "${SCRIPT_DIR}/docker-compose.local.yml" || -d "${SCRIPT_DIR}/data" ]]; then
    DEPLOY_DIR="${SCRIPT_DIR}"
    ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
    return 0
  fi

  if [[ -d "${SCRIPT_DIR}/deploy" ]]; then
    ROOT_DIR="${SCRIPT_DIR}"
    DEPLOY_DIR="${SCRIPT_DIR}/deploy"
    return 0
  fi

  return 1
}

if ! resolve_layout; then
  cat >&2 <<EOF
Could not locate the deploy directory for packaging.

Run this script in one of these ways:
  1. From the project root:
     ./deploy/package-server-bundle.sh --stop-stack --restart-stack
  2. From inside the deploy directory:
     ./package-server-bundle.sh --stop-stack --restart-stack
  3. From a directory that contains deploy/ and a copied package-server-bundle.sh
EOF
  exit 1
fi

if [[ -z "${OUTPUT_PATH}" ]]; then
  OUTPUT_PATH="${ROOT_DIR}/sub2api-server-bundle-${TIMESTAMP}.tar.gz"
fi

detect_compose_file() {
  local candidates=(
    "${DEPLOY_DIR}/docker-compose.local.yml"
    "${DEPLOY_DIR}/docker-compose.yml"
    "${DEPLOY_DIR}/docker-compose.dev.yml"
  )

  if [[ -n "${MANUAL_COMPOSE_FILE}" ]]; then
    echo "${MANUAL_COMPOSE_FILE}"
    return 0
  fi

  for candidate in "${candidates[@]}"; do
    if [[ -f "${candidate}" ]] && docker compose -f "${candidate}" ps -q 2>/dev/null | grep -q .; then
      echo "${candidate}"
      return 0
    fi
  done

  for candidate in "${candidates[@]}"; do
    if [[ -f "${candidate}" ]]; then
      echo "${candidate}"
      return 0
    fi
  done

  return 1
}

detect_package_compose_source() {
  local candidates=(
    "${DEPLOY_DIR}/docker-compose.local.yml"
    "${DEPLOY_DIR}/docker-compose.yml"
  )

  for candidate in "${candidates[@]}"; do
    if [[ -f "${candidate}" ]]; then
      echo "${candidate}"
      return 0
    fi
  done

  return 1
}

compose_uses_local_dirs() {
  local compose_file="$1"

  grep -Eq '\./data\s*:/app/data' "${compose_file}" &&
    grep -Eq '\./postgres_data\s*:/var/lib/postgresql/data' "${compose_file}" &&
    grep -Eq '\./redis_data\s*:/data' "${compose_file}"
}

if ! COMPOSE_FILE="$(detect_compose_file)"; then
  cat >&2 <<EOF
Could not find a docker compose file in ${DEPLOY_DIR}.

Expected one of:
  - ${DEPLOY_DIR}/docker-compose.local.yml
  - ${DEPLOY_DIR}/docker-compose.yml
  - ${DEPLOY_DIR}/docker-compose.dev.yml
EOF
  exit 1
fi

if ! PACKAGE_COMPOSE_SOURCE="$(detect_package_compose_source)"; then
  cat >&2 <<EOF
Could not find a local-directory compose file in ${DEPLOY_DIR}.

Expected one of:
  - ${DEPLOY_DIR}/docker-compose.local.yml
  - ${DEPLOY_DIR}/docker-compose.yml
EOF
  exit 1
fi

if ! compose_uses_local_dirs "${PACKAGE_COMPOSE_SOURCE}"; then
  cat >&2 <<EOF
The compose file at ${PACKAGE_COMPOSE_SOURCE} does not appear to use the local-directory migration layout.

This packaging script only supports deployments that keep data in:
  - deploy/data
  - deploy/postgres_data
  - deploy/redis_data
EOF
  exit 1
fi

required_paths=(
  "${DEPLOY_DIR}/.env"
  "${DEPLOY_DIR}/data"
  "${DEPLOY_DIR}/postgres_data"
  "${DEPLOY_DIR}/redis_data"
)

for path in "${required_paths[@]}"; do
  if [[ ! -e "$path" ]]; then
    echo "Missing required path: $path" >&2
    exit 1
  fi
done

stack_running=false
if docker compose -f "${COMPOSE_FILE}" ps -q 2>/dev/null | grep -q .; then
  stack_running=true
fi

if [[ "${stack_running}" == "true" && "${STOP_STACK}" != "true" ]]; then
  cat >&2 <<'EOF'
The local stack appears to be running.

For a consistent PostgreSQL data copy, stop the stack before packaging.
Re-run with:

  ./deploy/package-server-bundle.sh --stop-stack --restart-stack

This will:
1. Stop the local dev stack
2. Build a migration tarball
3. Start the local stack again
EOF
  exit 1
fi

cleanup() {
  if [[ -n "${STAGE_DIR:-}" && -d "${STAGE_DIR}" ]]; then
    rm -rf "${STAGE_DIR}"
  fi
}
trap cleanup EXIT

if [[ "${stack_running}" == "true" && "${STOP_STACK}" == "true" ]]; then
  echo "Stopping local stack for a consistent snapshot..."
  docker compose -f "${COMPOSE_FILE}" down
fi

STAGE_DIR="$(mktemp -d)"
mkdir -p "${STAGE_DIR}/deploy"

cp "${DEPLOY_DIR}/.env" "${STAGE_DIR}/deploy/.env"
cp "${PACKAGE_COMPOSE_SOURCE}" "${STAGE_DIR}/deploy/docker-compose.yml"
cp "${PACKAGE_COMPOSE_SOURCE}" "${STAGE_DIR}/deploy/docker-compose.local.yml"
cp "${SCRIPT_PATH}" "${STAGE_DIR}/deploy/package-server-bundle.sh"
cp -R "${DEPLOY_DIR}/data" "${STAGE_DIR}/deploy/data"
cp -R "${DEPLOY_DIR}/postgres_data" "${STAGE_DIR}/deploy/postgres_data"
cp -R "${DEPLOY_DIR}/redis_data" "${STAGE_DIR}/deploy/redis_data"

cat > "${STAGE_DIR}/deploy/DEPLOY_ON_SERVER.txt" <<'EOF'
1. Copy this archive to the server.
2. Extract it on the server:
   tar xzf sub2api-server-bundle-*.tar.gz
3. Enter the deploy directory:
   cd deploy
4. Review .env:
   - Set BIND_HOST=0.0.0.0 if you want direct IP:8080 access
   - Remove local-only Tailscale/Clash variables if they are not needed
5. Start the stack:
   docker compose up -d
6. Check status:
   docker compose ps
   docker compose logs -f sub2api
7. If this machine later becomes the source machine for the next migration, run from this deploy directory:
   ./package-server-bundle.sh --stop-stack --restart-stack
EOF

mkdir -p "$(dirname "${OUTPUT_PATH}")"
tar czf "${OUTPUT_PATH}" -C "${STAGE_DIR}" deploy

if [[ "${stack_running}" == "true" && "${STOP_STACK}" == "true" && "${RESTART_STACK}" == "true" ]]; then
  echo "Restarting local stack..."
  docker compose -f "${COMPOSE_FILE}" up -d
fi

echo "Bundle created at:"
echo "  ${OUTPUT_PATH}"
