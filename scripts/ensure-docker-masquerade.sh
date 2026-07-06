#!/usr/bin/env bash
set -euo pipefail

NETWORK_NAME="${1:-new-api_new-api-network}"

if [[ "$(id -u)" != "0" ]]; then
  echo "error: this script must run as root" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "error: docker command not found" >&2
  exit 1
fi

if ! command -v iptables >/dev/null 2>&1; then
  echo "error: iptables command not found" >&2
  exit 1
fi

subnets="$(
  docker network inspect "$NETWORK_NAME" \
    --format '{{range .IPAM.Config}}{{println .Subnet}}{{end}}' |
    sed '/^[[:space:]]*$/d'
)"

if [[ -z "$subnets" ]]; then
  echo "error: docker network '$NETWORK_NAME' has no IPv4 subnet" >&2
  exit 1
fi

sysctl -w net.ipv4.ip_forward=1 >/dev/null

while IFS= read -r subnet; do
  [[ -z "$subnet" ]] && continue
  if [[ "$subnet" == *:* ]]; then
    echo "skip IPv6 subnet: $subnet"
    continue
  fi

  if iptables -t nat -C POSTROUTING -s "$subnet" ! -d "$subnet" -j MASQUERADE 2>/dev/null; then
    echo "masquerade rule already exists for $subnet"
    continue
  fi

  iptables -t nat -A POSTROUTING -s "$subnet" ! -d "$subnet" -j MASQUERADE
  echo "added masquerade rule for $subnet"
done <<< "$subnets"
