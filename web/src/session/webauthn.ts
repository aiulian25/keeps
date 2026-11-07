// Converts between the server's JSON (base64url) WebAuthn options and the
// browser's ArrayBuffer-based credential API.

const BASE64_GROUP_LENGTH = 4;

function fromBase64Url(value: string): ArrayBuffer {
  const base64 = value
    .replace(/-/g, "+")
    .replace(/_/g, "/")
    .padEnd(
      Math.ceil(value.length / BASE64_GROUP_LENGTH) * BASE64_GROUP_LENGTH,
      "=",
    );
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes.buffer;
}

function toBase64Url(buffer: ArrayBuffer): string {
  let binary = "";
  for (const byte of new Uint8Array(buffer)) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary)
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}

type Json = Record<string, unknown>;

export function passkeysSupported() {
  return (
    typeof window.PublicKeyCredential !== "undefined" && window.isSecureContext
  );
}

export async function createPasskey(options: { publicKey: Json }) {
  const publicKey = options.publicKey as Json & {
    challenge: string;
    user: Json & { id: string };
    excludeCredentials?: Array<Json & { id: string }>;
  };
  const credential = (await navigator.credentials.create({
    publicKey: {
      ...(publicKey as unknown as PublicKeyCredentialCreationOptions),
      challenge: fromBase64Url(publicKey.challenge),
      user: {
        ...(publicKey.user as unknown as PublicKeyCredentialUserEntity),
        id: fromBase64Url(publicKey.user.id),
      },
      excludeCredentials: (publicKey.excludeCredentials ?? []).map(
        (descriptor) => ({
          ...(descriptor as unknown as PublicKeyCredentialDescriptor),
          id: fromBase64Url(descriptor.id),
        }),
      ),
    },
  })) as PublicKeyCredential | null;
  if (!credential) {
    throw new Error("No passkey was created.");
  }
  const response = credential.response as AuthenticatorAttestationResponse;
  return {
    id: credential.id,
    rawId: toBase64Url(credential.rawId),
    type: credential.type,
    response: {
      clientDataJSON: toBase64Url(response.clientDataJSON),
      attestationObject: toBase64Url(response.attestationObject),
      transports: response.getTransports?.() ?? [],
    },
  };
}

export async function getPasskey(options: { publicKey: Json }) {
  const publicKey = options.publicKey as Json & {
    challenge: string;
    allowCredentials?: Array<Json & { id: string }>;
  };
  const credential = (await navigator.credentials.get({
    publicKey: {
      ...(publicKey as unknown as PublicKeyCredentialRequestOptions),
      challenge: fromBase64Url(publicKey.challenge),
      allowCredentials: (publicKey.allowCredentials ?? []).map(
        (descriptor) => ({
          ...(descriptor as unknown as PublicKeyCredentialDescriptor),
          id: fromBase64Url(descriptor.id),
        }),
      ),
    },
  })) as PublicKeyCredential | null;
  if (!credential) {
    throw new Error("No passkey was used.");
  }
  const response = credential.response as AuthenticatorAssertionResponse;
  return {
    id: credential.id,
    rawId: toBase64Url(credential.rawId),
    type: credential.type,
    response: {
      clientDataJSON: toBase64Url(response.clientDataJSON),
      authenticatorData: toBase64Url(response.authenticatorData),
      signature: toBase64Url(response.signature),
      userHandle: response.userHandle ? toBase64Url(response.userHandle) : null,
    },
  };
}
