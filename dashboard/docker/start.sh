#!/bin/sh
set -e

# Generate .env from container environment so Laravel's env() can read
# values injected by Kubernetes via secretKeyRef.
env | grep -E '^(APP_|ASSET_|CREEL_|LOG_|SESSION_|DB_)' > /app/.env

# Cache config, routes, and views for production performance.
php /app/artisan config:cache
php /app/artisan route:cache
php /app/artisan view:cache

exec supervisord -c /etc/supervisor/conf.d/supervisord.conf
