#!/usr/bin/env bash
set -euo pipefail

command="${1:-status}"
shift || true

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
image="${LIQUIBASE_IMAGE:-liquibase/liquibase:4.31.1}"
url="${LIQUIBASE_COMMAND_URL:-jdbc:postgresql://host.docker.internal:5432/pawit}"
username="${LIQUIBASE_COMMAND_USERNAME:-pawit}"
password="${LIQUIBASE_COMMAND_PASSWORD:-local-password}"

docker run --rm \
  -v "${repo_root}:/workspace:ro" \
  -w /workspace \
  "${image}" \
  --defaults-file=/workspace/db/liquibase/liquibase.properties \
  --url="${url}" \
  --username="${username}" \
  --password="${password}" \
  "${command}" "$@"
