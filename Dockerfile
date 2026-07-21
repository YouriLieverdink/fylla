# syntax=docker/dockerfile:1

# Stage 1a: front-end assets (Vite → public/build)
FROM node:22-slim AS assets
WORKDIR /app
COPY package.json package-lock.json ./
# ponytail: npm install, not `npm ci` — the committed lockfile is generated on
# macOS and omits linux-only transitive optionals (@emnapi/*), which `npm ci`
# rejects. install honors pinned versions and fills the platform gap. Switch
# back to `npm ci` if the lockfile is ever regenerated on linux.
RUN npm install --no-audit --no-fund
COPY vite.config.js ./
COPY resources ./resources
RUN npm run build

# Stage 1b: PHP dependencies (no dev, optimized autoloader)
FROM composer:2 AS vendor
WORKDIR /app
COPY . .
# --no-scripts: package:discover boots the app, which hits the DB (SettingsProvider);
# no DB exists at build time. Laravel regenerates the manifest on first runtime boot.
RUN composer install --no-dev --optimize-autoloader --no-interaction --prefer-dist --no-scripts

# Stage 2: runtime — FrankenPHP (classic mode, no Octane), arm64
FROM dunglas/frankenphp:php8.4
WORKDIR /app

# pcntl: signals for queue:work/schedule:work; pdo_sqlite: the DB; opcache: perf
RUN install-php-extensions pcntl pdo_sqlite opcache \
    && apt-get update \
    && apt-get install -y --no-install-recommends supervisor \
    && rm -rf /var/lib/apt/lists/*

# App code + vendored deps, then the built assets over public/
COPY --from=vendor /app /app
COPY --from=assets /app/public/build /app/public/build

COPY docker/supervisord.conf /etc/supervisor/conf.d/fylla.conf
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

EXPOSE 80
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["supervisord", "-c", "/etc/supervisor/conf.d/fylla.conf", "-n"]
