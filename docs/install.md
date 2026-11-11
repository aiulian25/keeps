# Installing Keeps

Keeps runs as one container on a **Linux amd64** host. Other architectures (ARM, Raspberry Pi) are not supported.

You need Docker Engine with the Compose plugin, and an **HTTPS address** for Keeps (for example `https://keeps.example.com`). HTTPS is required: browsers only allow sign-in cookies and passkeys on secure addresses.

Pick one of three setups:

| You have…                                                     | Use                                                           |
| ------------------------------------------------------------- | ------------------------------------------------------------- |
| A reverse proxy on this host already (Caddy, nginx, Traefik…) | [A. Behind your proxy](#a-behind-your-existing-reverse-proxy) |
| A domain pointing at this host, and nothing on ports 80/443   | [B. Keeps with Caddy](#b-keeps-with-caddy-automatic-https)    |
| Your own trusted certificate files and no proxy               | [C. Direct TLS](#c-direct-tls-with-your-own-certificate)      |

## A. Behind your existing reverse proxy

1. Download `docker-compose.yml` and `.env.example` from the repository into a new folder, and rename `.env.example` to `.env`:

   ```bash
   curl -fLO https://raw.githubusercontent.com/aiulian25/keeps/main/docker-compose.yml
   ```

   ```bash
   curl -fL -o .env https://raw.githubusercontent.com/aiulian25/keeps/main/.env.example && chmod 600 .env
   ```

2. In `.env`, set `APP_ORIGIN` to the exact HTTPS address people will type, with no trailing slash:

   ```dotenv
   APP_ORIGIN=https://keeps.example.com
   ```

3. Point your proxy at `http://127.0.0.1:6030`. By default Keeps listens only on this host's loopback interface. Allow large uploads: backups can be several gigabytes. Keeps accepts a slow restore upload as long as data keeps arriving (it stops one that sends nothing for 2 minutes, or runs over 12 hours), and lets a backup download take as long; set your proxy's body size and upload/download timeouts to allow the same (for nginx: `client_max_body_size`, `client_body_timeout`, `proxy_read_timeout`, `proxy_send_timeout`).
4. Start Keeps:

   ```bash
   docker compose up -d
   ```

5. Find the one-time setup code in the logs:

   ```bash
   docker compose logs keeps | grep "setup code"
   ```

6. Open your address. Enter the setup code, a household name, a username and a password.

That's it. Nothing else needs configuring: Keeps generates its own secrets.

**If your proxy runs in a container or on another machine**, it cannot reach `127.0.0.1` on this host. Set `KEEPS_BIND_ADDRESS` in `.env` to this machine's LAN address and point the proxy there, or put both on a shared Docker network, point the proxy at `http://keeps:6030` and remove the `ports:` entry.

**Client IP addresses.** Keeps ignores `X-Forwarded-For` by default. If you want per-client rate limiting behind your proxy, set `KEEPS_TRUSTED_PROXIES` to the proxy's address, for example `KEEPS_TRUSTED_PROXIES=127.0.0.1`.

## B. Keeps with Caddy (automatic HTTPS)

Use the files in `deploy/proxy/`. Caddy is an extra container: it obtains a certificate for your domain and forwards to Keeps.

1. Make sure your domain's DNS points at this host and that ports 80 and 443 are free.
2. Copy `deploy/proxy/compose.yaml`, `deploy/proxy/Caddyfile` and `deploy/proxy/.env.example` into a folder. Rename `.env.example` to `.env` and set `KEEPS_DOMAIN=keeps.example.com`.
3. Run `docker compose up -d`, then follow steps 5–6 of setup A.

## C. Direct TLS with your own certificate

Use `deploy/direct-tls/compose.yaml` with the `.env.example` from the repository root, renamed to `.env`. Keeps serves HTTPS itself on host port 443.

1. Put `fullchain.pem` and `privkey.pem` in a `certs` folder next to the compose file. They must be readable by user ID 74532 (`chmod 644`, or `chown 74532`).
2. Set `APP_ORIGIN` in `.env` to the hostname on the certificate.
3. Run `docker compose up -d`, then follow steps 5–6 of setup A.

Renew certificates by replacing the files, then run `docker compose restart keeps`.

## Passkeys and addresses

Passkeys belong to the hostname in `APP_ORIGIN`.

- They work at `https://keeps.example.com`.
- They do not work at `http://192.168.10.10` or any plain-HTTP address.
- Changing the hostname later makes existing passkeys stop working. Your password and authenticator still work, and you can add passkeys again.

## Storage

All data lives in the `keeps_data` volume, mounted at `/data`. Keep it on local disk: the SQLite database must not live on SMB or NFS shares. Backups can go anywhere.

**Using a host folder instead of a named volume.** Keeps runs as user 74532 and never starts as root to fix permissions. Prepare the folder first:

```bash
sudo mkdir -p /srv/keeps-data
```

```bash
sudo chown 74532:74532 /srv/keeps-data
```

```bash
sudo chmod 700 /srv/keeps-data
```

Then replace `keeps_data:/data` with `/srv/keeps-data:/data`. If the folder isn't writable, Keeps stops with a message saying so. Don't use `chmod 777`.

## Backups

Back up from the app (**Account → Data → Back up now**), or from the host on a schedule. For scheduled backups, use an [age](https://age-encryption.org) public key so no passphrase is stored on the server:

```bash
docker compose run --rm keeps backup --recipient age1yourpublickey...
```

The file appears in `/data/backups` inside the volume. **Copy it to another machine or disk**: a backup on the same disk does not survive a disk failure. For example, a daily systemd timer or cron job could run the command above, then copy the newest file elsewhere with `rsync`.

To check a backup:

```bash
docker compose run --rm -T keeps verify-backup /data/backups/<file>.keeps.age --passphrase-stdin < passphrase.txt
```

Use `--identity-file` instead if the backup was made with a key.

`verify-backup` checks the backup the way a restore would: every file against the backup's checksums, the database's integrity, and every photo and document the database refers to. It writes only the backup's database to a private temporary folder (in `$TMPDIR`, or `/tmp`) and always deletes it. It exits with an error for a wrong passphrase, a damaged backup, or a backup with missing or damaged files.

`backup` and `verify-backup` say **complete** only when every referenced photo and document is in the file, and every original matches the size and SHA-256 recorded when it was uploaded. If some were missing from storage or damaged, they still keep or check the backup, list the missing files, and exit with an error, so a scheduled job notices.

Backups and exports from the app are written completely on the server before the download starts. They need free space in the data volume roughly the size of the backup, and the prepared file is removed as soon as it is downloaded or after 15 minutes.

## Network hardening (recommended)

Keeps fetches public product pages when you paste a link. It refuses private and local addresses. You can also stop the container reaching your LAN at the network level, for example with firewall rules on the Docker bridge, so the app is never the weak point.

## Security defaults already in the compose file

The compose file already sets all of these; you don't need to change anything:

- runs as non-root user 74532
- read-only root filesystem
- all Linux capabilities dropped
- `no-new-privileges`
- memory, CPU and process limits
- no Docker socket
- no privileged mode

The image has no shell or package manager.

Rootless Docker is recommended where available. Keeps' own release checks have not yet been run on a rootless Docker host.

## Verifying release images

Each release image prints its own version:

```bash
docker run --rm --network none ghcr.io/aiulian25/keeps:<version> version
```
