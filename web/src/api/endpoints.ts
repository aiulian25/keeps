// Typed wrappers for every API endpoint, grouped by area.
import { request, uploadWithProgress } from "./client";
import type {
  Account,
  Attachment,
  AttachmentKind,
  BootstrapState,
  Bookmark,
  CeremonyStart,
  DocumentPage,
  Group,
  GroupKind,
  ImportDraft,
  Item,
  ItemInput,
  ItemCondition,
  ItemPage,
  PageSnapshot,
  Preset,
  PresetInput,
  Part,
  PartInput,
  PreparedFile,
  Reference,
  RestoreResult,
  Revision,
  RoomCosts,
  RoomDetails,
  SessionSummary,
  Theme,
  TotpSetup,
} from "./types";

export const sessionApi = {
  bootstrap: () =>
    request<BootstrapState>("GET", "/bootstrap", { quietUnauthorized: true }),
  claim: (body: {
    setupCode: string;
    householdName: string;
    username: string;
    password: string;
  }) =>
    request<{ signedIn: boolean }>("POST", "/bootstrap", {
      body,
      quietUnauthorized: true,
    }),
  signIn: (body: { username: string; password: string }) =>
    request<{ awaitingSecondFactor: boolean }>("POST", "/sessions", {
      body,
      quietUnauthorized: true,
    }),
  secondFactor: (body: { code?: string; recoveryCode?: string }) =>
    request<{ signedIn: boolean }>("POST", "/sessions/second-factor", {
      body,
      quietUnauthorized: true,
    }),
  passkeyBegin: () =>
    request<CeremonyStart>("POST", "/sessions/passkey/begin", {
      quietUnauthorized: true,
    }),
  passkeyFinish: (body: { challengeId: string; credential: unknown }) =>
    request<{ signedIn: boolean }>("POST", "/sessions/passkey/finish", {
      body,
      quietUnauthorized: true,
    }),
  current: () =>
    request<Account>("GET", "/session", { quietUnauthorized: true }),
  signOut: () =>
    request<undefined>("DELETE", "/session", { quietUnauthorized: true }),
  recover: (body: { code: string; newPassword: string }) =>
    request<{ signedIn: boolean }>("POST", "/recover", {
      body,
      quietUnauthorized: true,
    }),
};

export const accountApi = {
  update: (
    body: Partial<{
      displayName: string;
      theme: Theme;
      householdName: string;
      currency: string;
    }>,
  ) => request<Account>("PATCH", "/account", { body }),
  reauthenticate: (body: {
    password: string;
    code?: string;
    recoveryCode?: string;
  }) => request<undefined>("POST", "/account/reauthenticate", { body }),
  changePassword: (body: {
    password: string;
    newPassword: string;
    code?: string;
    recoveryCode?: string;
  }) => request<undefined>("POST", "/account/password", { body }),
  sessions: () => request<SessionSummary[]>("GET", "/sessions"),
  revokeSession: (id: number) =>
    request<undefined>("DELETE", `/sessions/${id}`),
  revokeOthers: () => request<undefined>("POST", "/sessions/revoke-others"),
  totpBegin: () => request<TotpSetup>("POST", "/account/totp"),
  totpConfirm: (code: string) =>
    request<{ recoveryCodes: string[] }>("POST", "/account/totp/confirm", {
      body: { code },
    }),
  totpDisable: () => request<undefined>("DELETE", "/account/totp"),
  recoveryCodes: () =>
    request<{ recoveryCodes: string[] }>("POST", "/account/recovery-codes"),
  passkeyBegin: () => request<CeremonyStart>("POST", "/account/passkeys/begin"),
  passkeyFinish: (body: {
    challengeId: string;
    name: string;
    credential: unknown;
  }) => request<Account>("POST", "/account/passkeys/finish", { body }),
  passkeyRename: (id: number, name: string) =>
    request<Account>("PATCH", `/account/passkeys/${id}`, { body: { name } }),
  passkeyRemove: (id: number) =>
    request<Account>("DELETE", `/account/passkeys/${id}`),
};

export interface ItemQuery {
  q?: string;
  room?: number;
  category?: number;
  tag?: number;
  collection?: number;
  unassigned?: boolean;
  hasReceipt?: boolean;
  priceKnown?: boolean;
  condition?: ItemCondition;
  inWarranty?: boolean;
  deleted?: boolean;
  sort?: "newest" | "purchased" | "name";
  cursor?: string;
  limit?: number;
}

function queryString(query: ItemQuery) {
  const parameters = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === "" || value === false || value === 0) {
      continue;
    }
    parameters.set(key, value === true ? "1" : String(value));
  }
  const text = parameters.toString();
  return text ? `?${text}` : "";
}

export const itemsApi = {
  roomCosts: () => request<RoomCosts>("GET", "/items/room-costs"),
  list: (query: ItemQuery, signal?: AbortSignal) =>
    request<ItemPage>("GET", `/items${queryString(query)}`, { signal }),
  get: (id: number) => request<Item>("GET", `/items/${id}`),
  create: (input: Partial<ItemInput>, idempotencyKey: string) =>
    request<Item>("POST", "/items", { body: input, idempotencyKey }),
  update: (id: number, revision: number, input: ItemInput) =>
    request<Item>("PUT", `/items/${id}`, { body: input, revision }),
  history: (id: number) => request<Revision[]>("GET", `/items/${id}/history`),
  trash: (id: number) => request<undefined>("DELETE", `/items/${id}`),
  restore: (id: number) => request<undefined>("POST", `/items/${id}/restore`),
  purge: (id: number) => request<undefined>("DELETE", `/trash/${id}`),
  addPart: (id: number, part: PartInput) =>
    request<Part>("POST", `/items/${id}/parts`, { body: part }),
  updatePart: (id: number, partId: number, part: PartInput) =>
    request<Part>("PUT", `/items/${id}/parts/${partId}`, { body: part }),
  removePart: (id: number, partId: number) =>
    request<undefined>("DELETE", `/items/${id}/parts/${partId}`),
  linkAttachments: (id: number, attachmentIds: number[]) =>
    request<undefined>("POST", `/items/${id}/attachments`, {
      body: { attachmentIds },
    }),
  unlinkAttachment: (id: number, attachmentId: number) =>
    request<undefined>("DELETE", `/items/${id}/attachments/${attachmentId}`),
  setPrimaryPhoto: (id: number, attachmentId: number) =>
    request<undefined>("PUT", `/items/${id}/primary-photo`, {
      body: { attachmentId },
    }),
};

export const groupsApi = {
  list: (kind: GroupKind) => request<Group[]>("GET", `/groups/${kind}`),
  create: (kind: GroupKind, name: string, details: RoomDetails = {}) =>
    request<Reference>("POST", `/groups/${kind}`, {
      body: { name, ...details },
    }),
  rename: (kind: GroupKind, id: number, name: string) =>
    request<Reference>("PATCH", `/groups/${kind}/${id}`, { body: { name } }),
  update: (
    kind: GroupKind,
    id: number,
    changes: RoomDetails & { name?: string },
  ) => request<Reference>("PATCH", `/groups/${kind}/${id}`, { body: changes }),
  remove: (kind: GroupKind, id: number, reassignTo?: number) =>
    request<undefined>(
      "DELETE",
      `/groups/${kind}/${id}${reassignTo ? `?reassignTo=${reassignTo}` : ""}`,
    ),
};

export const presetsApi = {
  list: () => request<Preset[]>("GET", "/presets"),
  create: (preset: PresetInput) =>
    request<Preset>("POST", "/presets", { body: preset }),
  remove: (id: number) => request<undefined>("DELETE", `/presets/${id}`),
};

export const attachmentsApi = {
  bookmarks: (id: number) =>
    request<Bookmark[]>("GET", `/attachments/${id}/bookmarks`),
  addBookmark: (id: number, bookmark: Omit<Bookmark, "id">) =>
    request<Bookmark>("POST", `/attachments/${id}/bookmarks`, {
      body: bookmark,
    }),
  removeBookmark: (id: number, bookmarkId: number) =>
    request<undefined>("DELETE", `/attachments/${id}/bookmarks/${bookmarkId}`),
  upload: (
    kind: AttachmentKind,
    file: File,
    onProgress: (fraction: number) => void,
    signal?: AbortSignal,
  ) => {
    const form = new FormData();
    form.append("file", file);
    return uploadWithProgress<Attachment>(
      `/attachments?kind=${kind}`,
      form,
      onProgress,
      signal,
    );
  },
  rename: (id: number, filename: string) =>
    request<undefined>("PATCH", `/attachments/${id}`, { body: { filename } }),
  documents: (q: string, cursor = "") =>
    request<DocumentPage>(
      "GET",
      `/documents?${new URLSearchParams({ paged: "1", q, cursor })}`,
    ),
};

export const importsApi = {
  start: (url: string) =>
    request<ImportDraft>("POST", "/imports", { body: { url } }),
  startFromPage: (snapshot: PageSnapshot) =>
    request<ImportDraft>("POST", "/imports/page", { body: snapshot }),
  get: (id: string, signal?: AbortSignal) =>
    request<ImportDraft>("GET", `/imports/${id}`, { signal }),
  cancel: (id: string) => request<undefined>("DELETE", `/imports/${id}`),
  save: (
    id: string,
    candidate: number,
    item: ItemInput,
    idempotencyKey: string,
  ) =>
    request<Item>("POST", `/imports/${id}/save`, {
      body: { candidate, item },
      idempotencyKey,
    }),
};

export const dataApi = {
  prepareBackup: (passphrase: string) =>
    request<PreparedFile>("POST", "/backups", { body: { passphrase } }),
  prepareExport: (format: "csv" | "archive") =>
    request<PreparedFile>("POST", "/exports", { body: { format } }),
  restore: (
    file: File,
    passphrase: string,
    acceptIncomplete: boolean,
    onProgress: (fraction: number) => void,
  ) => {
    const form = new FormData();
    form.append("passphrase", passphrase);
    if (acceptIncomplete) {
      form.append("acceptIncomplete", "1");
    }
    form.append("file", file);
    return uploadWithProgress<RestoreResult>("/restore", form, onProgress);
  },
};
