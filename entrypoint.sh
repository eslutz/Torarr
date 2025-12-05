#!/bin/sh
set -e

echo "Starting Torarr..."

# Generate Tor control password if not set
if [ -z "$TOR_CONTROL_PASSWORD" ]; then
    export TOR_CONTROL_PASSWORD="torarr$(date +%s | sha256sum | base64 | head -c 16)"
    echo "Generated Tor control password: $TOR_CONTROL_PASSWORD"
fi

# Update torrc with hashed password
if [ -n "$TOR_CONTROL_PASSWORD" ]; then
    HASHED_PASSWORD=$(tor --hash-password "$TOR_CONTROL_PASSWORD" | tail -n 1)
    sed -i "s|^HashedControlPassword.*|HashedControlPassword $HASHED_PASSWORD|" /etc/tor/torrc
fi

# Start health server in background
echo "Starting health server..."
/usr/local/bin/healthserver &
HEALTH_PID=$!

# Trap signals for graceful shutdown
trap "echo 'Shutting down...'; kill -TERM $HEALTH_PID 2>/dev/null || true; exit 0" SIGTERM SIGINT

# Start Tor as main process
echo "Starting Tor..."
exec tor -f /etc/tor/torrc
