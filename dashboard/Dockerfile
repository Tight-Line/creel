FROM php:8.4-fpm AS base

RUN apt-get update && apt-get install -y --no-install-recommends \
    libzip-dev \
    nginx \
    supervisor \
    unzip \
    && docker-php-ext-install zip pcntl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=composer:2 /usr/bin/composer /usr/bin/composer

FROM base AS node
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM base AS vendor
WORKDIR /app
COPY composer.json composer.lock ./
RUN composer install --no-dev --optimize-autoloader --no-scripts

FROM base
WORKDIR /app

COPY . .
COPY --from=vendor /app/vendor ./vendor
COPY --from=node /app/public/build ./public/build

COPY docker/nginx.conf /etc/nginx/sites-enabled/default
COPY docker/supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY docker/php-fpm.conf /usr/local/etc/php-fpm-custom.conf
COPY docker/start.sh /usr/local/bin/start.sh

RUN chown -R www-data:www-data storage bootstrap/cache \
    && chown -R www-data:www-data /var/lib/nginx /var/log/nginx /run \
    && chown -R www-data:www-data /var/log/supervisor

EXPOSE 3000
USER www-data
CMD ["start.sh"]
