# Getting back in and recovering data

## Lost your authenticator?

1. On the sign-in page, enter your username and password.
2. At the code step, choose **Use a recovery code instead**.
3. Enter one of the recovery codes you saved. Each code works once.

Then go to **Account → Security** and set up the authenticator again.

## Lost everything (password, authenticator and recovery codes)

Whoever controls the server can reset the administrator. You don't need a shell inside the container.

1. Stop Keeps:

   ```bash
   docker compose stop keeps
   ```

2. Ask for a recovery code. The first run prints the instance fingerprint and changes nothing:

   ```bash
   docker compose run --rm keeps recover-admin
   ```

3. Run it again with that fingerprint to confirm:

   ```bash
   docker compose run --rm keeps recover-admin --confirm-instance <fingerprint>
   ```

4. Start Keeps:

   ```bash
   docker compose start keeps
   ```

5. Open `https://<your address>/recover`. Enter the code and choose a new password. The code works once and expires after 15 minutes.

Recovery signs everyone out and removes the authenticator, recovery codes and passkeys. Set them up again under **Account → Security**.

This relies on control of the server. Anyone who can run Docker commands on the host can do it, so protect host access.

## Setup code expired before you finished setup

```bash
docker compose run --rm keeps bootstrap-code
```

This works only before the administrator is created.

## Restoring a backup

**From the app**: **Account → Data → Restore**. Choose the `.keeps.age` file and enter its passphrase.

Keeps checks the whole file before changing anything. If the file is damaged, from a newer version, or the passphrase is wrong, nothing changes.

After a restore, everyone is signed out. Sign in with the account as it was in the backup.

**On a new or broken server** (Keeps stopped):

```bash
docker compose stop keeps
```

```bash
docker compose run --rm -T -v "$PWD/keeps-backup.keeps.age:/restore.keeps.age:ro" keeps restore /restore.keeps.age --passphrase-stdin < passphrase.txt
```

```bash
docker compose start keeps
```

Use `--identity-file` for backups made with an age key.

**A backup with missing files.** If a photo or document was already missing from storage when the backup was made, the backup can't contain it. Keeps lists each one (file name and item) and asks before restoring: in the app, choose **Restore without them**; on the command line, add `--accept-incomplete`. Everything else is restored, so keep incomplete backups rather than deleting them.

The previous data is kept in a `rollback-<time>` folder inside the data volume. Once you have checked the restore, delete it to free space.

Passkeys only work if Keeps is still served from the same hostname as when they were added.

## A restore went wrong

If a restore fails after it started replacing data, Keeps moves your previous data back and checks that it opens. When that works you see "your previous data was put back", and nothing else is needed.

If Keeps can't confirm that, it stops serving requests and shows "recovery needed", so nothing is changed further. Both copies are kept:

- The previous data is in a `rollback-<time>` folder inside the data volume.
- The command line prints the exact folder.

To recover by hand:

1. Stop Keeps and copy the whole data volume somewhere safe first.
2. Move `keeps.db`, `keys/` and `blobs/` from the `rollback-<time>` folder back to the top of the data volume, replacing the ones there, and delete any `keeps.db-wal` and `keeps.db-shm` files next to `keeps.db`.
3. Start Keeps and sign in. Delete the `rollback-<time>` folder only after checking your items, photos and documents.
