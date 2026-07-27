# Standup — Reverb in the FrankenPHP/supervisord image (#94)

Part of the wayfinder map #92 (Live activity over WebSockets). This is the
`wayfinder:task` branch: the config below **was actually built and run** on
arm64, not sketched. No application code changed — no event class, no emit
point, no client. Those are #97 and #95.

**Versions.** `laravel/reverb` **v1.11.0** (`composer require laravel/reverb`),
`laravel/framework` v13.18.0, base image `dunglas/frankenphp:php8.4`
(PHP 8.4.23), Docker 29.6.1 on darwin/arm64 (native build, no QEMU).

---

## Verdict

**It holds.** Reverb runs as a fourth `supervisord` program, survives a
`SIGKILL` in ~1s, idles at ~58 MB, serves `/up` for the healthcheck, and a
browser-side WebSocket client on the host receives an event published from
inside the container. Nothing in the runtime needed restructuring — no
Caddyfile, no reverse proxy, no extra base-image packages.

## The config that worked

`docker/supervisord.conf` — fourth program, same log/restart block as the
other three:

```ini
; WebSockets — Reverb, binds 0.0.0.0:8080 from config/reverb.php
[program:reverb]
command=php artisan reverb:start
autorestart=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
stderr_logfile=/dev/stderr
stderr_logfile_maxbytes=0
```

No `--host`/`--port` flags: `config/reverb.php` already defaults
`servers.reverb.host` to `0.0.0.0` and `port` to `8080`
(`REVERB_SERVER_HOST`/`REVERB_SERVER_PORT` if they ever need overriding).

`Dockerfile` — `EXPOSE 80 8080` (documentation only; the publish does the work).

`docker-compose.yml`:

```yaml
ports:
    - "1083:80"
    - "1084:8080"                      # Reverb WebSockets (#92)
environment:
    BROADCAST_CONNECTION: reverb
    REVERB_HOST: 127.0.0.1
    REVERB_PORT: 8080
    REVERB_SCHEME: http
healthcheck:
    test: ["CMD-SHELL", "curl -fsS http://localhost:80/ && curl -fsS http://localhost:8080/up"]
```

## Env keys: which live where

| Key | Value | Lives in | Why |
| --- | --- | --- | --- |
| `REVERB_APP_ID` | random 6-digit | `.env` (bind-mounted, #75) | per-install credential |
| `REVERB_APP_KEY` | random 20-char | `.env` | credential; also the value the *client* connects with |
| `REVERB_APP_SECRET` | random 20-char | `.env` | genuine secret — signs the publish request |
| `BROADCAST_CONNECTION` | `reverb` | compose `environment:` | deploy-fixed; default is `null`, which silently swallows every broadcast |
| `REVERB_HOST` | `127.0.0.1` | compose `environment:` | **publisher target**, see the trap below |
| `REVERB_PORT` | `8080` | compose `environment:` | default is `443` |
| `REVERB_SCHEME` | `http` | compose `environment:` | default is `https` |

`php artisan reverb:install` generates the three app credentials into `.env`
itself. It does **not** set `BROADCAST_CONNECTION` when the key is absent from
`.env` (it only rewrites an existing line) — so the connection defaults to
`null` and nothing broadcasts. Setting it in compose covers the deploy; a dev
`.env` needs the line added by hand.

### The `REVERB_HOST` trap (a constraint on #95)

`REVERB_HOST` is overloaded. It feeds:

- `broadcasting.connections.reverb.options.host` — where the **server-side
  publisher** sends its HTTP POST. Inside the container this must be
  `127.0.0.1`.
- `reverb.apps.apps.0.options.host` — advertised client options.
- `reverb.servers.reverb.hostname` — harmless here: the only use is TLS
  certificate lookup (`Servers/Reverb/Factory.php:119-124`), skipped without TLS.
- `VITE_REVERB_HOST` (via `"${REVERB_HOST}"` in the generated `.env`) — the
  **client's** connect host, which must be the *host machine*, never
  `127.0.0.1`-in-container.

So the publisher's host and the client's host cannot be the same key in this
deploy. #95 must derive the client host from something else — `window.location.hostname`
plus a separately-configured port is the obvious candidate — and leave
`REVERB_HOST=127.0.0.1` alone for the server side.

## What was verified, and how

Built `docker build -t fylla:reverb-probe .` natively on arm64 and ran it with
the compose env, on throwaway ports `1183:80` / `1184:8080` (the real deploy
already owns `1083`).

1. **Boot** — all four programs reach `RUNNING`:
   `frankenphp`, `queue`, `reverb`, `schedule`. Reverb logs
   `INFO Starting server on 0.0.0.0:8080 (127.0.0.1).`
2. **Connect from the host** — a plain Node `WebSocket` to
   `ws://localhost:1184/app/<REVERB_APP_KEY>?protocol=7&client=probe&version=1.0`
   gets `pusher:connection_established`, and
   `{"event":"pusher:subscribe","data":{"channel":"activity"}}` gets
   `pusher_internal:subscription_succeeded`. No auth involved — public channel,
   `allowed_origins => ['*']`.
3. **Publish from inside the container** reaches that client:
   ```
   docker exec fylla-probe php artisan tinker --execute="app('Illuminate\Contracts\Broadcasting\Factory')->connection('reverb')->broadcast(['activity'],'ActivityChanged',['from'=>'container']); echo 'sent';"
   ```
   → host client received
   `{"event":"ActivityChanged","data":"{\"from\":\"container\"}","channel":"activity"}`.
   This is the whole path #92 depends on, minus the event class.
4. **Restart survival** — `kill -9` on the Reverb pid:
   `WARN exited: reverb (terminated by SIGKILL; not expected)` →
   `INFO spawned: 'reverb' with pid 147` **1 second later**, RUNNING at +2s.
   The connected client saw close code `1006`; a fresh connect immediately after
   succeeded. That 1s gap is exactly what the disconnect tell renders.
5. **Healthcheck** — Reverb serves its own health route on the WS port
   (`Servers/Reverb/Factory.php:106`, `Route::get('/up', HealthCheckController)`),
   returning `{"health":"OK"}` with 200. `curl` is already in the image, so the
   compound `CMD-SHELL` above works as-is; exit 0 confirmed by `docker exec`.
   Note the base image's own `HEALTHCHECK` (Caddy admin `:2019`, disabled here)
   still applies to a bare `docker run` — only compose overrides it.

**Recommendation on the healthcheck: cover the WS port.** The map already
decided there is no poll floor, so a live Reverb is load-bearing for `/activity`.
`supervisord` restarts the process, so the probe is a signal rather than a cure,
but a container reporting healthy while the activity page is frozen is a lie.

## Idle memory

RSS after boot, four programs plus supervisord (container total 225 MB):

| Process | RSS |
| --- | --- |
| `frankenphp php-server` | 157 MB |
| `php artisan reverb:start` | **58 MB** |
| `php artisan schedule:work` | 54 MB |
| `php artisan queue:work` | 53 MB |
| `supervisord` | 28 MB |

Reverb is ordinary — the same order as the two artisan daemons already running.
No surprise for the box.

## Side effects of `composer require laravel/reverb`

`php artisan reverb:install --no-interaction` **crashes partway** with
`NonInteractiveValidationException: Required.` from a `Laravel\Prompts` confirm
inside the nested `install:broadcasting`. It is not atomic: by the time it
throws it has already written the `.env` keys, published `config/reverb.php`,
created `config/broadcasting.php` and `routes/channels.php`, and added
`channels:` to `bootstrap/app.php`. Run it interactively, or accept the partial
result and set `BROADCAST_CONNECTION` by hand (what this branch did).

**`routes/channels.php` and `/broadcasting/auth` are dead weight here, and this
branch drops them.** The app has no auth and the activity channel is public, so
the generated `App.Models.User.{id}` channel and the auth route serve nothing.
Verified after reverting the `channels:` line in `bootstrap/app.php` and leaving
`routes/channels.php` uncommitted: app boots, `route:list` has no broadcasting
route, 169 tests pass, and a public-channel publish still reaches a subscribed
client.

`composer audit` reports 4 medium `guzzlehttp/guzzle` advisories, all
**pre-existing** — guzzle was already in `composer.lock` before this branch
(`pusher/pusher-php-server` merely also requires it). Unrelated to #92.

Full suite after the install: **169 passed, 765 assertions**.

## Left for other tickets

- The event class, its emit point, and `ShouldBroadcastNow`/`ShouldRescue` — #97 (and #93's findings).
- How the client learns host/port/key — #95, constrained by the `REVERB_HOST` trap above.
- Whether `composer dev` gains a `reverb:start` process, and the README lines for it — still fog on the map.
