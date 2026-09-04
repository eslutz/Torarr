#!/bin/sh
set -e

echo "Starting Torarr..."

# Generate Tor control password if not set. The value is deliberately not
# logged: container logs are routinely shipped to log stores, and the control
# port can drive Tor. Set TOR_CONTROL_PASSWORD yourself if you need to know it.
if [ -z "$TOR_CONTROL_PASSWORD" ]; then
    TOR_CONTROL_PASSWORD="torarr$(head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 24)"
    export TOR_CONTROL_PASSWORD
    echo "Generated a random Tor control password (set TOR_CONTROL_PASSWORD to choose your own)"
fi

# Update torrc with hashed password
if [ -n "$TOR_CONTROL_PASSWORD" ]; then
    HASHED_PASSWORD=$(tor --hash-password "$TOR_CONTROL_PASSWORD" | tail -n 1)
    sed -i "s|^HashedControlPassword.*|HashedControlPassword $HASHED_PASSWORD|" /etc/tor/torrc
fi

# Configure Exit Nodes if specified
if [ -n "$TOR_EXIT_NODES" ]; then
    echo "ExitNodes $TOR_EXIT_NODES" >> /etc/tor/torrc
    echo "StrictNodes 1" >> /etc/tor/torrc
    echo "Configured ExitNodes: $TOR_EXIT_NODES"
fi

# Start health server in background, and refuse to continue if it did not
# start: a container with Tor up but no health server would look alive while
# every probe failed, which is exactly how a wrong-architecture binary hid.
echo "Starting health server..."
/usr/local/bin/healthserver &
HEALTH_PID=$!
sleep 1
if ! kill -0 "$HEALTH_PID" 2>/dev/null; then
    echo "Health server failed to start (is /usr/local/bin/healthserver built for this architecture?)" >&2
    exit 1
fi

# Trap signals for graceful shutdown
trap 'echo "Shutting down..."; kill -TERM $HEALTH_PID 2>/dev/null || true; exit 0' TERM INT

# Start Tor as main process
echo "Starting Tor..."
exec tor -f /etc/tor/torrc
