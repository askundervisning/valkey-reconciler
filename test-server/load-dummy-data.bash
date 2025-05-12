#!/bin/bash

# --- Configuration ---
# Name of your Valkey (or Redis) Service in Kubernetes
# This should be the Service that targets your primary Valkey pod(s)
# If you have both a Headless and a ClusterIP service, the ClusterIP service name is usually fine for this.
VALKEY_SERVICE_NAME="vk-valkey-primary" # <-- CHANGE THIS to your Valkey Service name
VALKEY_PORT="6379"                     # <-- CHANGE THIS if your Valkey port is different

# Data Generation Parameters
NUM_KEYS=100000       # Number of keys to set (adjust for total data size)
VALUE_SIZE_BYTES=1024 # Size of each value in bytes (1024 bytes = 1KB)

# Expected total data size (rough estimate): NUM_KEYS * VALUE_SIZE_BYTES
# Example: 100,000 keys * 1024 bytes/key = 102,400,000 bytes ≈ 97.6 MB
# Adjust NUM_KEYS or VALUE_SIZE_BYTES to get your desired size (+100MB).

# --- Script ---

echo "--- Valkey Dummy Data Loader ---"
echo "Target Service: $VALKEY_SERVICE_NAME:$VALKEY_PORT"
echo "Generating $NUM_KEYS keys with $VALUE_SIZE_BYTES bytes per value."
echo "Estimated total data size: $((NUM_KEYS * VALUE_SIZE_BYTES / 1024 / 1024)) MB"
echo "----------------------------------"

# Generate the dummy value string (a simple way to create a string of a specific size)
# We'll use 'x' repeatedly.
echo "Generating dummy value string..."
DUMMY_VALUE=$(printf '%*s' "$VALUE_SIZE_BYTES" | tr ' ' 'x')
echo "Dummy value string generated."
echo "----------------------------------"

echo "Generating SET commands and piping to redis-cli --pipe in a temporary Kubernetes pod..."

# Use a loop to generate SET commands and pipe them
# into 'kubectl run' which creates a temporary pod with redis-cli.
# 'redis-cli --pipe' is the fastest way to load data in bulk.
#
# kubectl run:
# -i: Keep stdin open for piping
# --rm: Automatically delete the pod after the command finishes
# --image=redis:latest: Use the official redis image which contains redis-cli
# --: Separates kubectl arguments from the command to run inside the pod
# The command inside the pod: redis-cli --pipe -h <service> -p <port>
#    --pipe: Enable pipe mode for bulk loading
#    -h <service>: Connect to the Valkey service name (resolves via internal DNS)
#    -p <port>: Connect on the specified port

(
  # Generate SET commands
  for i in $(seq 0 $((NUM_KEYS - 1))); do
    key="dummydata:$i"
    # redis-cli --pipe expects commands in a specific format (e.g., SET key value)
    # We just print the command line by line
    echo "SET $key $DUMMY_VALUE"
  done
) | \
kubectl run -i --rm dummy-data-loader --image=redis:latest -- \
  redis-cli -a DUcIhRTqjK --pipe --tls --insecure -h "$VALKEY_SERVICE_NAME" -p "$VALKEY_PORT"

# Check the exit status of the kubectl run command
if [ $? -eq 0 ]; then
  echo "----------------------------------"
  echo "Data loading process initiated successfully."
  echo "Check the output above for redis-cli --pipe report (Processed, Errors, etc.)."
else
  echo "----------------------------------"
  echo "Error: Data loading process failed."
  echo "Check kubectl and redis-cli output for details."
fi

echo "Script finished."