#!/bin/sh
set -e

# storage/ is not persisted (cache/session/queue use the database driver, #75),
# so recreate the skeleton the framework expects on every boot.
mkdir -p \
  storage/framework/cache/data \
  storage/framework/sessions \
  storage/framework/views \
  storage/logs \
  bootstrap/cache \
  /data/db

# .env (with APP_KEY + DB_DATABASE) is bind-mounted read-only at runtime (#75).
php artisan migrate --force

exec "$@"
