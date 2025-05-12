#!/bin/bash

# --- Configuration ---
URL="http://test-server.local:8000/"
INTERVAL=0.5 # Interval in seconds
LOG_FORMAT="%Y-%m-%d %H:%M:%S" # Log timestamp format (e.g., 2023-10-27 10:30:00)
# Use "%Y-%m-%d %H:%M:%S.%3N" for milliseconds if your system supports it

# --- Script ---

# Trap Ctrl+C to exit gracefully
trap "echo -e '\nExiting health check...'; exit" INT

echo "Starting health check on $URL every $INTERVAL seconds."
echo "Press Ctrl+C to stop."
echo "-------------------------------------------------"

i=0
while true; do
  # Get current timestamp
  timestamp=$(date +"$LOG_FORMAT")

  # Run curl:
  # -s: Silent mode, don't show progress or error messages
  # -o /dev/null: Discard the response body
  # -w "%{http_code}": Print only the HTTP status code
  # 2>/dev/null: Redirect curl's potential stderr output (errors) to /dev/null
#  status_code=$(curl -s -o /dev/null -w "%{http_code}" "$URL" 2>/dev/null)
  status_code=$(curl -s "$URL?value=$i" 2>/dev/null)
  i=$((i+1))

  # Check curl's exit status ($?) to see if the command itself failed (e.g., connection refused, DNS error)
  if [ $? -eq 0 ]; then
    # Curl command succeeded, now check the HTTP status code output
    if [[ -n "$status_code" && "$status_code" != "000" ]]; then
      # Status code is valid (not empty or 000)
      echo "$timestamp - Status: $status_code"
    else
      # Status code is empty or 000, often indicates a non-HTTP error after connection
      echo "$timestamp - Warning: curl succeeded, but returned empty/000 status. May indicate connection issues."
    fi
  else
    # Curl command failed
    echo "$timestamp - Error: curl command failed."
  fi

  # Wait for the specified interval
  sleep "$INTERVAL"
done
