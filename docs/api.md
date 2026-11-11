# Keeps API v1

The web app and any future client (for example a Linux desktop app) use the same JSON API under `/api/v1` on the configured `APP_ORIGIN`.

## Stability promise

Within `v1`, changes are additive only: new endpoints, new optional request fields and new response fields. Clients must ignore response fields they do not know. Removing or renaming anything requires `/api/v2`.

## Conventions

- **Requests and responses** are JSON (`Content-Type: application/json`), except uploads (`multipart/form-data`) and downloads.
- **Unknown request fields are rejected** with `400 invalid_request`, so a client can never set ownership or role fields by accident.
- **Errors** always have this shape:

  ```json
  {
    "code": "invalid_item",
    "message": "Check the highlighted fields.",
    "fields": { "name": "Enter a name." }
  }
  ```

  `code` is stable and meant for programs; `message` is plain English for people; `fields` is present for field-level problems.

- **Authentication** is a server-side session cookie (`__Host-keeps_session`, `HttpOnly`, `Secure`, `SameSite=Lax`).
- **CSRF**: every `POST`, `PUT`, `PATCH` and `DELETE` needs:
  - an `Origin` header exactly equal to `APP_ORIGIN`, and
  - an `X-CSRF-Token` header with the token from `GET /api/v1/session`. The download endpoints also accept it as a `csrfToken` form field.
- **Recent confirmation**: some endpoints need the user to have confirmed their identity within the last 10 minutes (`POST /api/v1/account/reauthenticate`). Otherwise they return `403 reauthentication_required`. These are marked † below.
- **Concurrency**: `PUT /api/v1/items/{id}` requires `If-Match: "<revision>"`. A stale revision returns `409 edit_conflict`.
- **Idempotency**: `POST /api/v1/items` and `POST /api/v1/imports/{id}/save` accept an `Idempotency-Key` header. Repeating a request with the same key returns the item created the first time.
- **Money** is a decimal string plus an ISO 4217 code, for example `{ "amount": "79.00", "currency": "GBP" }`. Unknown is `null`, never `0`. Use a dot for decimals. A comma is accepted only as a thousands separator in an amount that also has a decimal dot (`1,234.50`); other commas (`79,99`, `1,234`) are refused with a `pricePaid` field error rather than guessed.
- **Dates**: `{ "value": "2024-03-12", "precision": "date" }` or `{ "value": "2024-03-12T14:30", "precision": "datetime" }`. Unknown is `null`.

## Health (no authentication, no data)

| Method | Path       | Purpose                                           |
| ------ | ---------- | ------------------------------------------------- |
| GET    | `/healthz` | Liveness: `ok`                                    |
| GET    | `/readyz`  | Readiness: database open and data volume writable |

## Setup and sign-in (public, origin-checked)

| Method | Path                              | Purpose                                                                                                           |
| ------ | --------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| GET    | `/api/v1/bootstrap`               | `{claimed, passkeysAvailable}`                                                                                    |
| POST   | `/api/v1/bootstrap`               | Claim the instance: `{setupCode, householdName, username, password}`. Returns `410 setup_code_used` once claimed. |
| POST   | `/api/v1/sessions`                | Password sign-in: `{username, password}` → `{awaitingSecondFactor}`                                               |
| POST   | `/api/v1/sessions/second-factor`  | `{code}` or `{recoveryCode}` completes a pending sign-in                                                          |
| POST   | `/api/v1/sessions/passkey/begin`  | Starts a discoverable passkey login → `{challengeId, options}`                                                    |
| POST   | `/api/v1/sessions/passkey/finish` | `{challengeId, credential}`                                                                                       |
| POST   | `/api/v1/recover`                 | `{code, newPassword}` using a code from `keeps recover-admin`                                                     |

## Session and account

| Method | Path                                | Purpose                                                                   |
| ------ | ----------------------------------- | ------------------------------------------------------------------------- |
| GET    | `/api/v1/session`                   | Current account, including `csrfToken`                                    |
| DELETE | `/api/v1/session`                   | Sign out                                                                  |
| GET    | `/api/v1/sessions`                  | Active sessions                                                           |
| DELETE | `/api/v1/sessions/{id}`             | Sign out one session                                                      |
| POST   | `/api/v1/sessions/revoke-others`    | Sign out every other session                                              |
| GET    | `/api/v1/account`                   | Same as `GET /session`                                                    |
| PATCH  | `/api/v1/account`                   | `{displayName?, theme?, householdName?, currency?}`                       |
| POST   | `/api/v1/account/reauthenticate`    | `{password, code?, recoveryCode?}`                                        |
| POST   | `/api/v1/account/password`          | `{password, newPassword, code?, recoveryCode?}`; signs out other sessions |
| POST   | `/api/v1/account/totp` †            | Begin authenticator setup → `{secret, uri, qrCode}`                       |
| POST   | `/api/v1/account/totp/confirm` †    | `{code}` → `{recoveryCodes}` (shown once)                                 |
| DELETE | `/api/v1/account/totp` †            | Turn off the authenticator                                                |
| POST   | `/api/v1/account/recovery-codes` †  | New recovery codes (old ones stop working)                                |
| POST   | `/api/v1/account/passkeys/begin` †  | Begin passkey registration                                                |
| POST   | `/api/v1/account/passkeys/finish` † | `{challengeId, name, credential}`                                         |
| PATCH  | `/api/v1/account/passkeys/{id}`     | `{name}`                                                                  |
| DELETE | `/api/v1/account/passkeys/{id}` †   | Remove a passkey                                                          |

## Items

| Method | Path                                            | Purpose                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| ------ | ----------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| GET    | `/api/v1/items`                                 | List. Query: `q, room, category, tag, collection, condition, inWarranty=1, unassigned=1, hasReceipt=1, priceKnown=1, deleted=1, sort=newest\|purchased\|name, cursor, limit` → `{items, total, nextCursor}`                                                                                                                                                                                                                                                |
| GET    | `/api/v1/items/totals`                          | Price-paid totals per currency (never summed across currencies)                                                                                                                                                                                                                                                                                                                                                                                            |
| GET    | `/api/v1/items/room-costs`                      | Price paid × quantity per room, across all rooms, and for items not in a room: `{rooms: {<id>: {totals, pricedItems, unpricedItems}}, allRooms, unassigned}`; totals are per currency, never summed across currencies                                                                                                                                                                                                                                      |
| POST   | `/api/v1/items`                                 | Create; only `name` is required. Response includes `possibleDuplicates`. Optional `measurements`: `{width, height, depth, lengthUnit (mm                                                                                                                                                                                                                                                                                                                   | cm  | m   | in  | ft), weight, weightUnit (g | kg  | lb  | oz), fitNote}`with values as decimal strings above 0, kept exactly; a unit is required for any value given, and all-blank clears them. With`purchaseId`, the item joins that purchase: blank `seller`, `orderReference`and`purchaseDate` keep the stored values, and stated ones replace them for every item sharing it. Editing an item that already belongs to the purchase (`PUT`) replaces its facts, including clearing them. An unknown purchase returns 404 `purchase_not_found`. |
| GET    | `/api/v1/items/{id}`                            | Full item (`ETag` = revision)                                                                                                                                                                                                                                                                                                                                                                                                                              |
| PUT    | `/api/v1/items/{id}`                            | Replace all editable fields (`If-Match` required)                                                                                                                                                                                                                                                                                                                                                                                                          |
| GET    | `/api/v1/items/{id}/history`                    | Change history                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| DELETE | `/api/v1/items/{id}`                            | Move to Recently deleted                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| POST   | `/api/v1/items/{id}/parts`                      | Record a compatible part or consumable: `{name, partNumber, note, sourceUrl}` (name required; `sourceUrl` is an http(s) reference, never fetched). Parts appear in the item as `parts`.                                                                                                                                                                                                                                                                    |
| PUT    | `/api/v1/items/{id}/parts/{partId}`             | Replace a recorded part                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| DELETE | `/api/v1/items/{id}/parts/{partId}`             | Remove a recorded part                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| POST   | `/api/v1/items/{id}/units`                      | Identify one unit of an item bought as several: `{label, serialNumber}` (at least one). Units appear in the item as `units`, in the order added, and are found by search. The item keeps its quantity, price, purchase and receipts. Refused with `all_units_identified` once every unit of the quantity is identified, or `too_many_units` past 100. Lowering an item's quantity below its identified units returns `quantity_below_units` on `quantity`. |
| PUT    | `/api/v1/items/{id}/units/{unitId}`             | Replace a unit's label and serial number                                                                                                                                                                                                                                                                                                                                                                                                                   |
| DELETE | `/api/v1/items/{id}/units/{unitId}`             | Remove an identified unit                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| POST   | `/api/v1/items/{id}/restore`                    | Restore from Recently deleted                                                                                                                                                                                                                                                                                                                                                                                                                              |
| DELETE | `/api/v1/trash/{id}`                            | Permanently delete an item that is already in Recently deleted                                                                                                                                                                                                                                                                                                                                                                                             |
| POST   | `/api/v1/items/{id}/attachments`                | `{attachmentIds}`: invoices attach to the item's purchase, so they are shared with other items from the same purchase                                                                                                                                                                                                                                                                                                                                      |
| DELETE | `/api/v1/items/{id}/attachments/{attachmentId}` | Detach a file                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| PUT    | `/api/v1/items/{id}/primary-photo`              | `{attachmentId}`                                                                                                                                                                                                                                                                                                                                                                                                                                           |

## Rooms, categories, tags, collections

`{kind}` is `rooms`, `categories`, `tags` or `collections`.

| Method | Path                         | Purpose                                                                                 |
| ------ | ---------------------------- | --------------------------------------------------------------------------------------- |
| GET    | `/api/v1/groups/{kind}`      | `[{id, name, itemCount, thumbnailIds}]`; rooms also include `description` and `photoId` |
| POST   | `/api/v1/groups/{kind}`      | `{name}`; rooms also accept optional `description` and `photoId`                        |
| PATCH  | `/api/v1/groups/{kind}/{id}` | `{name?}`; rooms also accept `description?` and `photoId?` (`0` removes the photo)      |
| DELETE | `/api/v1/groups/{kind}/{id}` | Optional `?reassignTo=<id>` (rooms and categories). Items are never deleted.            |

## Files

| Method | Path                                              | Purpose                                                                                                                                                                                                                                                                                                                                                  |
| ------ | ------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| POST   | `/api/v1/attachments`                             | `?kind=photo\|invoice\|document`, multipart field `file`. JPEG, PNG, WebP or PDF, up to 20 MB.                                                                                                                                                                                                                                                           |
| GET    | `/api/v1/attachments/{id}`                        | `?variant=original\|preview\|thumbnail`. Originals and PDFs are always downloads.                                                                                                                                                                                                                                                                        |
| PATCH  | `/api/v1/attachments/{id}`                        | `{filename}`                                                                                                                                                                                                                                                                                                                                             |
| GET    | `/api/v1/attachments/{id}/bookmarks`              | Named pages of a document: `[{id, page, label}]`, ordered by page                                                                                                                                                                                                                                                                                        |
| POST   | `/api/v1/attachments/{id}/bookmarks`              | `{page, label}`: name a page (label up to 120 characters; page from 1). The viewer flags bookmarks beyond the file's page count.                                                                                                                                                                                                                         |
| DELETE | `/api/v1/attachments/{id}/bookmarks/{bookmarkId}` | Remove a bookmark                                                                                                                                                                                                                                                                                                                                        |
| GET    | `/api/v1/documents`                               | Invoices and documents across items, one row per item a file belongs to: `?q=`. Without `paged`, the original array of up to 500 rows, newest first. With `?paged=1` (and `cursor` from the previous page): `{documents, total, nextCursor}`, 100 per page, in a stable order (time, then attachment, then item); a bad cursor returns `invalid_cursor`. |

## Link import

| Method | Path                        | Purpose                                                                                                                                                           |
| ------ | --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| POST   | `/api/v1/imports`           | `{url}` → draft `{id, state}`. Unusable links fail immediately (`invalid_url`, `not_public`, `private_document_link`).                                            |
| POST   | `/api/v1/imports/page`      | `{url, title, metaTags, structuredData}` read by the "Add to Keeps" browser button from the page the user is viewing; no request is made to the shop for the page |
| GET    | `/api/v1/imports/{id}`      | Draft with `state` (`queued`, `running`, `done`, `failed`, `cancelled`, `saved`), `errorCode`, `candidates`                                                       |
| DELETE | `/api/v1/imports/{id}`      | Cancel                                                                                                                                                            |
| POST   | `/api/v1/imports/{id}/save` | `{candidate, item}`: creates the item. Listing data is stored separately from the price paid.                                                                     |

## Presets

| Method | Path                   | Purpose                                                                                                                                                                                                               |
| ------ | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| GET    | `/api/v1/presets`      | Saved starting details for new items: `[{id, name, room, category, tags, collections, condition, notes, missing}]`. `missing` names groups that don't exist (yet or any more); saving an item with them creates them. |
| POST   | `/api/v1/presets`      | `{name, room, category, tags, collections, condition, notes}`. Only these fields: never serial numbers, prices, purchases or files. A name is required and unique; at least one detail must be filled.                |
| DELETE | `/api/v1/presets/{id}` | Delete a preset                                                                                                                                                                                                       |

## Data

| Method | Path                       | Purpose                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ------ | -------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| POST   | `/api/v1/exports` †        | `{format: "csv" \| "archive"}`. Prepares the whole file first, then returns `{downloadUrl, filename, complete, missingFiles}`. The archive holds `items.json` (items, plus `groups`: every room with description and `photoFile`, category, tag and collection, including empty ones), `files/` (originals, each once) and `checksums.sha256`; it leaves out Recently deleted, history, unlinked uploads and credentials. A file missing from storage fails the archive with `export_file_missing`; a full disk returns 507 `not_enough_space`. |
| POST   | `/api/v1/backups` †        | `{passphrase}`. Prepares the encrypted backup (`.keeps.age`) first, then returns the same shape. `complete: false` lists each `missingFiles` entry as `{filename, copy, items}`; the backup is still offered, for recovery.                                                                                                                                                                                                                                                                                                                     |
| GET    | `/api/v1/downloads/{id}` † | Fetches a prepared file once, as an attachment. It expires after 15 minutes; afterwards `download_expired`.                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| POST   | `/api/v1/restore` †        | Multipart: `passphrase` and optionally `acceptIncomplete=1`, then `file`. A backup with missing files is refused with 409 `backup_incomplete` and `details.missingFiles` unless accepted. Signs everyone out.                                                                                                                                                                                                                                                                                                                                   |

## Notes for native clients

Sessions are cookie-based and tied to the browser. A future desktop client should not embed passwords. It should pair with the server through the system browser and keep a device token in the OS keyring. That pairing flow is deliberately not part of v1; see the decision record `docs/decisions/0005-native-clients.md` in the planning folder.
