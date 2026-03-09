#!/bin/sh
set -e

# Generate .env from container environment so Laravel's env() can read
# values injected by Kubernetes via secretKeyRef.
env | grep -E '^(APP_|CREEL_|LOG_|SESSION_|DB_)' > /app/.env

exec supervisord -c /etc/supervisor/conf.d/supervisord.conf
