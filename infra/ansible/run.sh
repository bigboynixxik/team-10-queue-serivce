#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if [[ ! -f .env ]]; then
  echo "Missing infra/.env — copy .env.example and fill HOST_IP, SSH_USER, SSH_PRIVATE_KEY" >&2
  exit 1
fi

set -a
source .env
set +a

if [[ -z "${HOST_IP:-}" || -z "${SSH_USER:-}" || -z "${SSH_PRIVATE_KEY:-}" ]]; then
  echo "HOST_IP, SSH_USER and SSH_PRIVATE_KEY must be set in .env" >&2
  exit 1
fi

if ! command -v ansible-playbook >/dev/null 2>&1; then
  echo "ansible-playbook not found. Install with: brew install ansible" >&2
  exit 1
fi

exec ansible-playbook "$@"
