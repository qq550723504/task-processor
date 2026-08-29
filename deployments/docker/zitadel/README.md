# Local ZITADEL

Local development stack based on the official ZITADEL Docker Compose setup.

## Self-contained local database

Start ZITADEL with the included Postgres service:

```powershell
Copy-Item .env.example .env
# Edit .env and set local-only ZITADEL_MASTERKEY / POSTGRES_ADMIN_PASSWORD.
docker compose --env-file .env -f docker-compose.yml up -d --wait
```

Open:

```text
http://localhost:8080
```

Initial login:

```text
zitadel-admin@zitadel.localhost
Password1!
```

The local `.env` file is ignored by git. Do not commit local master keys,
database passwords, PATs, or generated client secrets.

Stop:

```powershell
docker compose --env-file .env -f docker-compose.yml down
```

Reset all local ZITADEL data:

```powershell
docker compose --env-file .env -f docker-compose.yml down -v
```

## Yudao database

To preserve the existing Yudao mode, make sure the external
`docker_yudao-network` network exists and set `ZITADEL_DATABASE_POSTGRES_DSN`
in `.env` to the Yudao Postgres DSN. Then start with the explicit overlay:

```powershell
docker compose --env-file .env -f docker-compose.yml -f docker-compose.yudao-db.yml up -d --wait
```

The overlay keeps the included Postgres service behind the `local-db` profile,
connects ZITADEL to `docker_yudao-network`, and uses the external DSN from
`.env`.

Stop the Yudao-backed stack with the same file set:

```powershell
docker compose --env-file .env -f docker-compose.yml -f docker-compose.yudao-db.yml down
```

Reset ZITADEL-owned local volumes while using Yudao mode:

```powershell
docker compose --env-file .env -f docker-compose.yml -f docker-compose.yudao-db.yml down -v
```

The reset command does not delete the external Yudao database or its Docker
network.
