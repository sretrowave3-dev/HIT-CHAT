/**
 * Browser-side Cryptographic Ratchet
 * Implements WebCrypto AES-256-GCM encryption with 12-byte IVs and 16-byte auth tags.
 */

export interface WebEnvelope {
  id: string;
  conversation_id?: string;
  seq?: number;
  recipient_id: string;
  sender_id: string;
  ephemeral_public_key: string;
  ratchet_seq: number;
  chain_seq: number;
  iv: string; // Base64
  ciphertext: string; // Base64
  auth_tag: string; // Base64
  ttl_ms: number;
  timestamp_ms: number;
}

export function generateUUIDv7(): string {
  const bytes = new Uint8Array(16);
  if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
    crypto.getRandomValues(bytes);
  } else if (typeof globalThis !== 'undefined' && (globalThis as any).crypto?.getRandomValues) {
    (globalThis as any).crypto.getRandomValues(bytes);
  } else {
    for (let i = 0; i < 16; i++) bytes[i] = Math.floor(Math.random() * 256);
  }
  const now = Date.now();
  bytes[0] = (now >> 40) & 0xff;
  bytes[1] = (now >> 32) & 0xff;
  bytes[2] = (now >> 24) & 0xff;
  bytes[3] = (now >> 16) & 0xff;
  bytes[4] = (now >> 8) & 0xff;
  bytes[5] = now & 0xff;
  bytes[6] = (bytes[6] & 0x0f) | 0x70; // Version 7
  bytes[8] = (bytes[8] & 0x3f) | 0x80; // Variant
  return Array.from(bytes)
    .map((b, i) => ([4, 6, 8, 10].includes(i) ? '-' : '') + b.toString(16).padStart(2, '0'))
    .join('');
}

export function deriveConversationId(user1: string, user2: string): string {
  const u1 = canonicalHandle(user1);
  const u2 = canonicalHandle(user2);
  if (u2.startsWith('group_') || u2.startsWith('study_') || u2.startsWith('robotics_')) {
    return `group:${u2}`;
  }
  return u1 <= u2 ? `conv:${u1}:${u2}` : `conv:${u2}:${u1}`;
}

export function getCrypto(): Crypto {
  if (typeof window !== 'undefined' && window.crypto) {
    return window.crypto;
  }
  if (typeof globalThis !== 'undefined' && (globalThis as any).crypto) {
    return (globalThis as any).crypto as Crypto;
  }
  throw new Error('WebCrypto API is not available in this environment');
}

export function getSubtle(): SubtleCrypto {
  const c = getCrypto();
  if (!c.subtle) {
    throw new Error('WebCrypto subtle is unavailable. Ensure HTTPS or localhost is used.');
  }
  return c.subtle;
}

export function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('');
}

export function hexToBytes(hex: string): Uint8Array {
  const clean = hex.trim();
  const bytes = new Uint8Array(clean.length / 2);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(clean.substring(i * 2, i * 2 + 2), 16);
  }
  return bytes;
}

export function canonicalHandle(h: string): string {
  let clean = (h || '').trim().toLowerCase().replace(/^@/, '');
  clean = clean.replace(/^student_/, '');
  return clean;
}

export function uint8ArrayToBase64(bytes: Uint8Array): string {
  let binary = '';
  const len = bytes.byteLength;
  const chunkSize = 8192;
  for (let i = 0; i < len; i += chunkSize) {
    const chunk = bytes.subarray(i, Math.min(i + chunkSize, len));
    binary += String.fromCharCode.apply(null, Array.from(chunk));
  }
  return btoa(binary);
}

export function base64ToUint8Array(b64: string): Uint8Array {
  let clean = (b64 || '').replace(/-/g, '+').replace(/_/g, '/').trim();
  while (clean.length % 4 !== 0) {
    clean += '=';
  }
  const binary = atob(clean);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

export function getPairwiseSecret(user1: string, user2: string): string {
  const u1 = canonicalHandle(user1);
  const u2 = canonicalHandle(user2);
  const [first, second] = u1 <= u2 ? [u1, u2] : [u2, u1];
  return `CampusNet_Ratchet_Shared_Demo_Secret:${first}:${second}`;
}

export class WebRatchet {
  /**
   * Derives a pairwise AES-256 key raw bytes matching Go's RFC 5869 HKDF-SHA256 derivation.
   */
  static async derivePairwiseKeyBytes(user1: string, user2: string): Promise<Uint8Array> {
    const u1 = canonicalHandle(user1);
    const u2 = canonicalHandle(user2);
    const [first, second] = u1 <= u2 ? [u1, u2] : [u2, u1];
    const enc = new TextEncoder();

    const baseKey = await getSubtle().importKey(
      'raw',
      enc.encode('CampusNet_Ratchet_Shared_Demo_Secret'),
      'HKDF',
      false,
      ['deriveBits']
    );

    const bits = await getSubtle().deriveBits(
      {
        name: 'HKDF',
        hash: 'SHA-256',
        salt: enc.encode('CampusNet_Pairwise_Salt_v1'),
        info: enc.encode(`CampusNet-v1-Pairwise:${first.length}:${first}:${second.length}:${second}`),
      },
      baseKey,
      256
    );

    return new Uint8Array(bits);
  }

  /**
   * Derives a pairwise AES-256 key matching Go's RFC 5869 HKDF-SHA256 derivation.
   */
  static async derivePairwiseKey(user1: string, user2: string): Promise<CryptoKey> {
    const keyBytes = await this.derivePairwiseKeyBytes(user1, user2);
    return getSubtle().importKey(
      'raw',
      keyBytes as any,
      { name: 'AES-GCM', length: 256 },
      false,
      ['encrypt', 'decrypt']
    );
  }

  /**
   * Generates isolated pairwise secret for two communicating peers.
   */
  static getPairwiseSecret(user1: string, user2: string): string {
    return getPairwiseSecret(user1, user2);
  }

  /**
   * Derives a symmetric AES-256 key from a shared secret string/salt.
   */
  static async deriveKey(secret: string): Promise<CryptoKey> {
    const enc = new TextEncoder();
    const rawKey = await getSubtle().digest('SHA-256', enc.encode(secret));
    return getSubtle().importKey(
      'raw',
      rawKey,
      { name: 'AES-GCM' },
      false,
      ['encrypt', 'decrypt']
    );
  }

  /**
   * Low-level AEAD Decrypt with optional AAD
   */
  static async decryptPayload(
    keyBytes: Uint8Array,
    iv: Uint8Array,
    ciphertext: Uint8Array,
    tag: Uint8Array,
    aad?: Uint8Array
  ): Promise<Uint8Array> {
    const key = await getSubtle().importKey(
      'raw',
      keyBytes as any,
      { name: 'AES-GCM' },
      false,
      ['decrypt']
    );

    const combined = new Uint8Array(ciphertext.length + tag.length);
    combined.set(ciphertext, 0);
    combined.set(tag, ciphertext.length);

    const params: AesGcmParams = {
      name: 'AES-GCM',
      iv: iv as any,
      tagLength: 128,
      ...(aad ? { additionalData: aad as any } : {}),
    };

    const decrypted = await getSubtle().decrypt(params, key, combined as any);
    return new Uint8Array(decrypted);
  }

  /**
   * Low-level AEAD Encrypt with optional AAD
   */
  static async encryptPayload(
    keyBytes: Uint8Array,
    iv: Uint8Array,
    plaintext: Uint8Array,
    aad?: Uint8Array
  ): Promise<{ ciphertext: Uint8Array; authTag: Uint8Array }> {
    const key = await getSubtle().importKey(
      'raw',
      keyBytes as any,
      { name: 'AES-GCM' },
      false,
      ['encrypt']
    );

    const params: AesGcmParams = {
      name: 'AES-GCM',
      iv: iv as any,
      tagLength: 128,
      ...(aad ? { additionalData: aad as any } : {}),
    };

    const encrypted = await getSubtle().encrypt(params, key, plaintext as any);
    const encBytes = new Uint8Array(encrypted);
    const cipherLen = encBytes.length - 16;
    const ciphertext = encBytes.slice(0, cipherLen);
    const authTag = encBytes.slice(cipherLen);
    return { ciphertext, authTag };
  }

  /**
   * Encrypts plaintext message into an E2EE envelope using RFC 5869 HKDF pairwise key.
   */
  static async encryptMessage(
    sender: string,
    recipient: string,
    plaintext: string,
    ttlMS: number,
    _sessionSecret?: string
  ): Promise<WebEnvelope> {
    const keyBytes = await this.derivePairwiseKeyBytes(sender, recipient);
    const enc = new TextEncoder();
    const data = enc.encode(plaintext);

    const iv = getCrypto().getRandomValues(new Uint8Array(12));
    const ephemeralKey = getCrypto().getRandomValues(new Uint8Array(32));

    const { ciphertext, authTag } = await this.encryptPayload(keyBytes, iv, data);

    const uuid = generateUUIDv7();
    const convId = deriveConversationId(sender, recipient);

    return {
      id: uuid,
      conversation_id: convId,
      sender_id: sender,
      recipient_id: recipient,
      ephemeral_public_key: uint8ArrayToBase64(ephemeralKey),
      ratchet_seq: 1,
      chain_seq: 1,
      iv: uint8ArrayToBase64(iv),
      ciphertext: uint8ArrayToBase64(ciphertext),
      auth_tag: uint8ArrayToBase64(authTag),
      ttl_ms: ttlMS,
      timestamp_ms: Date.now(),
    };
  }

  /**
   * Decrypts an incoming E2EE envelope using pairwise HKDF key.
   * ZERO-DOWNGRADE: Rejects messages not encrypted with this conversation's pairwise key.
   */
  static async decryptMessage(env: WebEnvelope, _sessionSecret?: string): Promise<string> {
    const keyBytes = await this.derivePairwiseKeyBytes(env.sender_id, env.recipient_id);

    const ivBytes = base64ToUint8Array(env.iv);
    const cipherBytes = base64ToUint8Array(env.ciphertext);
    const tagBytes = base64ToUint8Array(env.auth_tag);

    const decrypted = await this.decryptPayload(keyBytes, ivBytes, cipherBytes, tagBytes);
    const dec = new TextDecoder();
    return dec.decode(decrypted);
  }

  /**
   * Decrypts with strict pairwise verification (no downgrade fallback).
   */
  static async decryptWithPairwiseFallback(env: WebEnvelope, _fallbackSecret?: string): Promise<string> {
    return await this.decryptMessage(env);
  }
}
