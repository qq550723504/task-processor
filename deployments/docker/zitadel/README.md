# Local ZITADEL

Local development stack based on the official ZITADEL Docker Compose setup.

Start:

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
