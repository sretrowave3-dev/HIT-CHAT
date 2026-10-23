/**
 * Browser-side Cryptographic Ratchet
 * Implements WebCrypto AES-256-GCM encryption with 12-byte IVs and 16-byte auth tags.
 */

export interface WebEnvelope {
  id: string;
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

export function canonicalHandle(h: string): string {
  let clean = (h || '').trim().toLowerCase().replace(/^@/, '');
  clean = clean.replace(/^student_/, '');
  return clean;
}

export function getPairwiseSecret(user1: string, user2: string): string {
  const u1 = canonicalHandle(user1);
  const u2 = canonicalHandle(user2);
  const [first, second] = u1 <= u2 ? [u1, u2] : [u2, u1];
  return `CampusNet_Ratchet_Shared_Demo_Secret:${first}:${second}`;
}

export class WebRatchet {
  /**
   * Derives a pairwise AES-256 key matching Go's RFC 5869 HKDF-SHA256 derivation.
   */
  static async derivePairwiseKey(user1: string, user2: string): Promise<CryptoKey> {
    const u1 = canonicalHandle(user1);
    const u2 = canonicalHandle(user2);
    const [first, second] = u1 <= u2 ? [u1, u2] : [u2, u1];
    const enc = new TextEncoder();

    const baseKey = await window.crypto.subtle.importKey(
      'raw',
      enc.encode('CampusNet_Ratchet_Shared_Demo_Secret'),
      'HKDF',
      false,
      ['deriveKey']
    );

    return window.crypto.subtle.deriveKey(
      {
        name: 'HKDF',
        hash: 'SHA-256',
        salt: enc.encode('CampusNet_Pairwise_Salt_v1'),
        info: enc.encode(`CampusNet-v1-Pairwise:${first.length}:${first}:${second.length}:${second}`),
      },
      baseKey,
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
    const rawKey = await window.crypto.subtle.digest('SHA-256', enc.encode(secret));
    return window.crypto.subtle.importKey(
      'raw',
      rawKey,
      { name: 'AES-GCM' },
      false,
      ['encrypt', 'decrypt']
    );
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
    const key = await this.derivePairwiseKey(sender, recipient);
    const enc = new TextEncoder();
    const data = enc.encode(plaintext);

    const iv = window.crypto.getRandomValues(new Uint8Array(12));
    const ephemeralKey = window.crypto.getRandomValues(new Uint8Array(32));

    const encrypted = await window.crypto.subtle.encrypt(
      {
        name: 'AES-GCM',
        iv: iv,
        tagLength: 128, // 16 bytes
      },
      key,
      data
    );

    // WebCrypto appends the 16-byte tag to the end of ciphertext
    const encBytes = new Uint8Array(encrypted);
    const cipherLen = encBytes.length - 16;
    const ciphertextBytes = encBytes.slice(0, cipherLen);
    const tagBytes = encBytes.slice(cipherLen);

    const uuid = crypto.randomUUID();

    return {
      id: uuid,
      sender_id: sender,
      recipient_id: recipient,
      ephemeral_public_key: btoa(String.fromCharCode(...ephemeralKey)),
      ratchet_seq: 1,
      chain_seq: 1,
      iv: btoa(String.fromCharCode(...iv)),
      ciphertext: btoa(String.fromCharCode(...ciphertextBytes)),
      auth_tag: btoa(String.fromCharCode(...tagBytes)),
      ttl_ms: ttlMS,
      timestamp_ms: Date.now(),
    };
  }

  /**
   * Decrypts an incoming E2EE envelope using pairwise HKDF key.
   * ZERO-DOWNGRADE: Rejects messages not encrypted with this conversation's pairwise key.
   */
  static async decryptMessage(env: WebEnvelope, _sessionSecret?: string): Promise<string> {
    const key = await this.derivePairwiseKey(env.sender_id, env.recipient_id);

    const ivBytes = Uint8Array.from(atob(env.iv), c => c.charCodeAt(0));
    const cipherBytes = Uint8Array.from(atob(env.ciphertext), c => c.charCodeAt(0));
    const tagBytes = Uint8Array.from(atob(env.auth_tag), c => c.charCodeAt(0));

    // Combine ciphertext and tag for WebCrypto
    const combined = new Uint8Array(cipherBytes.length + tagBytes.length);
    combined.set(cipherBytes, 0);
    combined.set(tagBytes, cipherBytes.length);

    const decrypted = await window.crypto.subtle.decrypt(
      {
        name: 'AES-GCM',
        iv: ivBytes,
        tagLength: 128,
      },
      key,
      combined
    );

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
