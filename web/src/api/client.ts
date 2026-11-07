// The one place that talks HTTP. Adds the CSRF header, maps errors to
// ApiError, and announces an expired session so forms can keep their data.

const API_PREFIX = "/api/v1";
const CSRF_HEADER = "X-CSRF-Token";
const IDEMPOTENCY_HEADER = "Idempotency-Key";
const IF_MATCH_HEADER = "If-Match";
export const STATUS_UNAUTHORIZED = 401;
const STATUS_NO_CONTENT = 204;
const STATUS_SUCCESS_MINIMUM = 200;
const STATUS_SUCCESS_LIMIT = 300;
export const SESSION_EXPIRED_EVENT = "keeps:session-expired";
export const OFFLINE_CODE = "offline";

let csrfToken = "";

export function setCsrfToken(token: string) {
  csrfToken = token;
}

export function getCsrfToken() {
  return csrfToken;
}

export class ApiError extends Error {
  status: number;
  code: string;
  fields: Record<string, string>;
  /** Extra data some errors carry, such as the files a backup lacks. */
  details: unknown;

  constructor(
    status: number,
    code: string,
    message: string,
    fields: Record<string, string> = {},
    details: unknown = undefined,
  ) {
    super(message);
    this.status = status;
    this.code = code;
    this.fields = fields;
    this.details = details;
  }
}

export interface RequestOptions {
  body?: unknown;
  idempotencyKey?: string;
  revision?: number;
  signal?: AbortSignal;
  /** Sign-in and setup endpoints handle 401 themselves. */
  quietUnauthorized?: boolean;
}

export async function request<T>(
  method: string,
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const headers: Record<string, string> = {};
  const isFormData = options.body instanceof FormData;
  if (options.body !== undefined && !isFormData) {
    headers["Content-Type"] = "application/json";
  }
  if (csrfToken) {
    headers[CSRF_HEADER] = csrfToken;
  }
  if (options.idempotencyKey) {
    headers[IDEMPOTENCY_HEADER] = options.idempotencyKey;
  }
  if (options.revision !== undefined) {
    headers[IF_MATCH_HEADER] = `"${options.revision}"`;
  }

  let response: Response;
  try {
    response = await fetch(API_PREFIX + path, {
      method,
      headers,
      credentials: "same-origin",
      signal: options.signal,
      body: bodyFor(options.body, isFormData),
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw error;
    }
    throw new ApiError(
      0,
      OFFLINE_CODE,
      "Couldn't reach Keeps. Your changes are still here.",
    );
  }

  if (response.status === STATUS_NO_CONTENT) {
    return undefined as T;
  }
  const payload = await response.json().catch(() => ({}));
  if (response.ok) {
    return payload as T;
  }
  const error = new ApiError(
    response.status,
    payload.code ?? "error",
    payload.message ?? "Something went wrong.",
    payload.fields ?? {},
    payload.details,
  );
  if (response.status === STATUS_UNAUTHORIZED && !options.quietUnauthorized) {
    window.dispatchEvent(new CustomEvent(SESSION_EXPIRED_EVENT));
  }
  throw error;
}

function bodyFor(body: unknown, isFormData: boolean): BodyInit | undefined {
  if (body === undefined) {
    return undefined;
  }
  if (isFormData) {
    return body as FormData;
  }
  return JSON.stringify(body);
}

export function newIdempotencyKey() {
  return crypto.randomUUID();
}

export function attachmentUrl(
  attachmentId: number,
  variant: "original" | "preview" | "thumbnail" = "original",
) {
  return `${API_PREFIX}/attachments/${attachmentId}?variant=${variant}`;
}

/**
 * Fetches a file the server has already prepared. A plain link streams it
 * straight to disk, so large backups never pass through JavaScript memory.
 */
export function startDownload(url: string) {
  const link = document.createElement("a");
  link.href = url;
  link.style.display = "none";
  document.body.appendChild(link);
  link.click();
  link.remove();
}

/** Uploads with progress (fetch has no upload progress). */
export function uploadWithProgress<T>(
  path: string,
  form: FormData,
  onProgress: (fraction: number) => void,
  signal?: AbortSignal,
): Promise<T> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", API_PREFIX + path);
    xhr.setRequestHeader(CSRF_HEADER, csrfToken);
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) {
        onProgress(event.loaded / event.total);
      }
    };
    xhr.onload = () => {
      const payload = safeParse(xhr.responseText);
      if (
        xhr.status >= STATUS_SUCCESS_MINIMUM &&
        xhr.status < STATUS_SUCCESS_LIMIT
      ) {
        resolve(payload as T);
        return;
      }
      if (xhr.status === STATUS_UNAUTHORIZED) {
        window.dispatchEvent(new CustomEvent(SESSION_EXPIRED_EVENT));
      }
      reject(
        new ApiError(
          xhr.status,
          payload.code ?? "error",
          payload.message ?? "The upload failed.",
          payload.fields ?? {},
          payload.details,
        ),
      );
    };
    xhr.onerror = () =>
      reject(
        new ApiError(
          0,
          OFFLINE_CODE,
          "The upload was interrupted. Check your connection and try again.",
        ),
      );
    xhr.onabort = () =>
      reject(new DOMException("Upload cancelled", "AbortError"));
    signal?.addEventListener("abort", () => xhr.abort());
    xhr.send(form);
  });
}

function safeParse(text: string) {
  try {
    return JSON.parse(text);
  } catch {
    return {};
  }
}
