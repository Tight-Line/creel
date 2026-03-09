#!/bin/sh
set -e

# Laravel's env() reads from .env via phpdotenv, not from system
# environment variables. Generate .env from the container env so
# config:cache picks up Kubernetes-injected values.
env | grep -E '^(APP_|CREEL_|LOG_|SESSION_|DB_)' > /app/.env

php /app/artisan config:cache

exec supervisord -c /etc/supervisor/conf.d/supervisord.conf
