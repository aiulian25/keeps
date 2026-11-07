# Upgrading Keeps

1. **Back up first**: **Account → Data → Back up now**, or run `docker compose run --rm keeps backup ...` (see install.md).
2. Pull and restart. The compose file follows `ghcr.io/aiulian25/keeps:0`, the newest 0.x release:

   ```bash
   docker compose pull
   ```

   ```bash
   docker compose up -d
   ```

3. Check the logs, then sign in:

   ```bash
   docker compose logs keeps
   ```

Before changing the database layout, Keeps saves a copy in `/data/backups/pre-migration-<version>-<time>.db`.

## Rolling back

Keeps refuses to open a database written by a newer version. It will not run an old binary on a newer schema. To go back:

1. Change the image line in `docker-compose.yml` to the exact version you had, for example `ghcr.io/aiulian25/keeps:0.1.0`.
2. Restore the backup you made before upgrading (see recovery.md).

Keeps never updates itself. Schema changes only happen when you choose to upgrade. To stay on one version, pin the image line to it (`:0.1.0`) instead of `:0`.
