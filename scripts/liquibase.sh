#!/usr/bin/env bash
set -euo pipefail

command="${1:-status}"
shift || true

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
properties_file="${repo_root}/db/liquibase/liquibase.properties"
changelog_file="${repo_root}/db/liquibase/changelog/db.changelog-root.yaml"
image="${LIQUIBASE_IMAGE:-liquibase/liquibase:4.31.1}"
url="${LIQUIBASE_COMMAND_URL:-jdbc:postgresql://host.docker.internal:5432/pawit}"
username="${LIQUIBASE_COMMAND_USERNAME:-pawit}"
password="${LIQUIBASE_COMMAND_PASSWORD:-local-password}"

usage() {
  cat <<'USAGE'
Usage: scripts/liquibase.sh [command] [args...]

Runs Liquibase in Docker using the repository changelog.

Common commands:
  validate     Validate the changelog
  status       Show pending changesets
  update       Apply pending changesets
  rollback     Roll back changesets; pass Liquibase rollback args after command

Environment:
  LIQUIBASE_IMAGE              Docker image, default liquibase/liquibase:4.31.1
  LIQUIBASE_COMMAND_URL        JDBC URL, default jdbc:postgresql://host.docker.internal:5432/pawit
  LIQUIBASE_COMMAND_USERNAME   Database username, default pawit
  LIQUIBASE_COMMAND_PASSWORD   Database password, default local-password
USAGE
}

if [[ "${command}" == "-h" || "${command}" == "--help" || "${command}" == "help" ]]; then
  usage
  exit 0
fi

if [[ ! -f "${properties_file}" ]]; then
  echo "Liquibase properties file not found: ${properties_file}" >&2
  exit 1
fi

if [[ ! -f "${changelog_file}" ]]; then
  echo "Liquibase changelog not found: ${changelog_file}" >&2
  exit 1
fi

docker run --rm \
  -v "${repo_root}:/workspace:ro" \
  -w /workspace \
  "${image}" \
  --defaults-file=/workspace/db/liquibase/liquibase.properties \
  --url="${url}" \
  --username="${username}" \
  --password="${password}" \
  "${command}" "$@"
