#!/usr/bin/env bash
# Create a STABLE self-signed code-signing identity for keysat so macOS keeps
# its Accessibility / Input Monitoring grant across rebuilds (ad-hoc signing
# changes identity every build and drops the grant). Idempotent.
set -euo pipefail
SIGN_ID="${1:-keysat-dev}"
KC="$HOME/Library/Keychains/keysat-signing.keychain-db"
KCPASS="keysat-local"

if security find-identity -p codesigning 2>/dev/null | grep -q "$SIGN_ID"; then
  echo "code-signing identity '$SIGN_ID' already exists — nothing to do."
  exit 0
fi

TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
cat > "$TMP/c.cnf" <<CNF
[req]
distinguished_name=dn
x509_extensions=v3
prompt=no
[dn]
CN=$SIGN_ID
[v3]
basicConstraints=critical,CA:false
keyUsage=critical,digitalSignature
extendedKeyUsage=critical,codeSigning
CNF
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$TMP/k.pem" -out "$TMP/c.pem" -days 3650 -config "$TMP/c.cnf" 2>/dev/null
openssl pkcs12 -export -inkey "$TMP/k.pem" -in "$TMP/c.pem" -out "$TMP/id.p12" -passout pass: -name "$SIGN_ID" 2>/dev/null

security delete-keychain "$KC" 2>/dev/null || true
security create-keychain -p "$KCPASS" "$KC"
security set-keychain-settings "$KC"
security unlock-keychain -p "$KCPASS" "$KC"
security import "$TMP/id.p12" -k "$KC" -P "" -A -T /usr/bin/codesign
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$KCPASS" "$KC" >/dev/null 2>&1 || true
# prepend our keychain to the user search list, preserving the rest
EXIST="$(security list-keychains -d user | sed -e 's/^[[:space:]]*"//' -e 's/"$//')"
security list-keychains -d user -s "$KC" $EXIST
echo "created code-signing identity '$SIGN_ID':"
security find-identity -p codesigning | grep "$SIGN_ID"
