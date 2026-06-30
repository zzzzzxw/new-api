#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT="${PORT:-3000}"
ENV_FILE="$ROOT_DIR/.env"

usage() {
  cat <<EOF
Usage: ./run-standalone.sh [--port PORT] [--skip-build]

Examples:
  ./run-standalone.sh
  ./run-standalone.sh --port 3000
  PORT=3000 ./run-standalone.sh
  ./run-standalone.sh --skip-build
EOF
}

SKIP_BUILD=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --port)
      if [ "$#" -lt 2 ]; then
        echo "Missing value for --port" >&2
        exit 1
      fi
      PORT="$2"
      shift 2
      ;;
    --skip-build)
      SKIP_BUILD=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

need_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing command: $1" >&2
    echo "Please install $1 first, then run this script again." >&2
    exit 1
  fi
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    date "+%s%N" | shasum -a 256 | awk '{print $1}'
    return
  fi
  date "+%s%N" | sha256sum | awk '{print $1}'
}

ensure_env() {
  if [ ! -f "$ENV_FILE" ]; then
    cat >"$ENV_FILE" <<EOF
SESSION_SECRET=$(random_secret)
SQLITE_PATH=./one-api.db
TZ=Asia/Shanghai
ERROR_LOG_ENABLED=true
BATCH_UPDATE_ENABLED=true
EOF
    echo "Created .env with a generated SESSION_SECRET."
    return
  fi

  if ! grep -qE '^[[:space:]]*SESSION_SECRET=' "$ENV_FILE"; then
    {
      echo ""
      echo "SESSION_SECRET=$(random_secret)"
    } >>"$ENV_FILE"
    echo "Added SESSION_SECRET to existing .env."
  fi
}

build_frontends() {
  need_command bun

  echo "Installing frontend dependencies..."
  cd "$ROOT_DIR/web"
  if [ -f bun.lock ]; then
    bun install --frozen-lockfile
  else
    bun install
  fi

  echo "Building default frontend..."
  cd "$ROOT_DIR/web/default"
  DISABLE_ESLINT_PLUGIN=true VITE_REACT_APP_VERSION="$(cat "$ROOT_DIR/VERSION" 2>/dev/null || true)" bun run build

  echo "Building classic frontend..."
  cd "$ROOT_DIR/web/classic"
  VITE_REACT_APP_VERSION="$(cat "$ROOT_DIR/VERSION" 2>/dev/null || true)" bun run build
}

need_command go
ensure_env

if [ "$SKIP_BUILD" -eq 0 ]; then
  build_frontends
else
  echo "Skipping frontend build."
fi

cd "$ROOT_DIR"
echo ""
echo "Starting new-api on http://localhost:$PORT"
echo "Database: ${SQLITE_PATH:-./one-api.db} unless SQL_DSN is configured in .env"
echo ""
exec go run . --port "$PORT"
