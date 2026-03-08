#!/bin/sh
set -e

# Re-cache config at runtime so env vars (APP_KEY, etc.) take effect.
php /app/artisan config:cache
php /app/artisan route:cache
php /app/artisan view:cache

exec supervisord -c /etc/supervisor/conf.d/supervisord.conf
