#!/bin/bash
# generate-tor-password.sh

PASS="test12345"
echo "Generating Tor control password..."
echo "Password: '$PASS'"

HASH=$(tor --hash-password "$PASS" 2>/dev/null)

echo "HashedControlPassword $HASH"
echo ""
echo "Add this line to your torrc file to enable authenticated control access."
