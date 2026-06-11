#!/usr/bin/env bash
# Generate a self-signed CA + collector TLS certificate for the production
# tokenmeter central collector stack.
#
# Usage:
#   ./generate-certs.sh [HOSTNAME]
#
# HOSTNAME defaults to the machine's hostname. Override when the collector is
# accessed via a domain name or IP address:
#   ./generate-certs.sh collector.internal
#   ./generate-certs.sh 10.0.0.5
#
# Output (./certs/):
#   ca.key         CA private key (keep secret, do not ship to edge machines)
#   ca.crt         CA certificate  (copy to each edge machine — trust anchor)
#   collector.key  Collector TLS private key
#   collector.crt  Collector TLS certificate (signed by CA)
set -euo pipefail

HOSTNAME="${1:-$(hostname)}"
DIR="$(cd "$(dirname "$0")" && pwd)/certs"
mkdir -p "$DIR"

# ── 1. CA ────────────────────────────────────────────────────────────────────
openssl genrsa -out "$DIR/ca.key" 4096 2>/dev/null
openssl req -new -x509 -days 3650 \
  -key "$DIR/ca.key" \
  -out "$DIR/ca.crt" \
  -subj "/CN=tokenmeter-ca/O=tokenmeter"

# ── 2. Collector key + CSR ───────────────────────────────────────────────────
openssl genrsa -out "$DIR/collector.key" 2048 2>/dev/null
openssl req -new \
  -key "$DIR/collector.key" \
  -out "$DIR/collector.csr" \
  -subj "/CN=${HOSTNAME}/O=tokenmeter"

# ── 3. SAN extension (supports IP SANs for bare-IP deployments) ──────────────
SAN="DNS:${HOSTNAME},DNS:localhost"
# If HOSTNAME looks like an IP, add an IP SAN too.
if [[ "$HOSTNAME" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  SAN="${SAN},IP:${HOSTNAME}"
fi
SAN="${SAN},IP:127.0.0.1"

openssl x509 -req -days 825 \
  -in "$DIR/collector.csr" \
  -CA "$DIR/ca.crt" -CAkey "$DIR/ca.key" -CAcreateserial \
  -out "$DIR/collector.crt" \
  -extfile <(printf "subjectAltName=%s\n" "$SAN") 2>/dev/null

rm "$DIR/collector.csr"
chmod 600 "$DIR/ca.key" "$DIR/collector.key"

echo ""
echo "✓ Certificates written to $DIR/"
echo ""
echo "  ca.crt         — copy to each edge machine (tls_ca_cert in config)"
echo "  collector.crt  — mounted into the collector container"
echo "  collector.key  — mounted into the collector container (keep secret)"
echo ""
echo "Edge tokenmeter config:"
echo "  sinks:"
echo "    otel:"
echo "      options:"
echo "        insecure: false"
echo "        tls_ca_cert: /path/to/ca.crt"
echo "        bearer_token: \$TOKENMETER_COLLECTOR_TOKEN"
