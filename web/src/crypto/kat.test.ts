import { describe, it, expect } from 'vitest';
import fixtures from '../../../testdata/crypto_kat_fixtures.json';
import {
  canonicalHandle,
  WebRatchet,
  bytesToHex,
  hexToBytes,
  base64ToUint8Array,
  uint8ArrayToBase64,
} from './ratchet';

describe('Known-Answer Tests (Cross-Implementation Go & WebCrypto)', () => {
  it('Handle Normalization matches Go CanonicalHandle', () => {
    for (const item of fixtures.handle_normalization) {
      const actual = canonicalHandle(item.input);
      expect(actual).toBe(item.expected);
    }
  });

  it('Pairwise HKDF Key Derivation matches Go RFC 5869 exact bytes', async () => {
    for (const item of fixtures.pairwise_kdf) {
      const derivedBytes = await WebRatchet.derivePairwiseKeyBytes(item.user1, item.user2);
      const derivedHex = bytesToHex(derivedBytes);
      expect(derivedHex).toBe(item.derived_key_hex);
    }
  });

  it('AEAD Decrypt Go-produced ciphertext with auth tag and AAD', async () => {
    const keyBytes = hexToBytes(fixtures.aead.key_hex);
    const iv = base64ToUint8Array(fixtures.aead.iv_b64);
    const ciphertext = base64ToUint8Array(fixtures.aead.ciphertext_b64);
    const tag = base64ToUint8Array(fixtures.aead.auth_tag_b64);
    const aad = new TextEncoder().encode(fixtures.aead.aad);

    const decryptedBytes = await WebRatchet.decryptPayload(keyBytes, iv, ciphertext, tag, aad);
    const decryptedText = new TextDecoder().decode(decryptedBytes);
    expect(decryptedText).toBe(fixtures.aead.plaintext);
  });

  it('AEAD Encrypt matches Go-produced ciphertext with same IV and AAD', async () => {
    const keyBytes = hexToBytes(fixtures.aead.key_hex);
    const iv = base64ToUint8Array(fixtures.aead.iv_b64);
    const plaintext = new TextEncoder().encode(fixtures.aead.plaintext);
    const aad = new TextEncoder().encode(fixtures.aead.aad);

    const { ciphertext, authTag } = await WebRatchet.encryptPayload(keyBytes, iv, plaintext, aad);
    expect(uint8ArrayToBase64(ciphertext)).toBe(fixtures.aead.ciphertext_b64);
    expect(uint8ArrayToBase64(authTag)).toBe(fixtures.aead.auth_tag_b64);
  });

  it('Round-Trip: TS encrypts and Go decrypts matches ts_aead fixture', async () => {
    const keyBytes = hexToBytes(fixtures.ts_aead.key_hex);
    const iv = base64ToUint8Array(fixtures.ts_aead.iv_b64);
    const plaintext = new TextEncoder().encode(fixtures.ts_aead.plaintext);
    const aad = new TextEncoder().encode(fixtures.ts_aead.aad);

    const { ciphertext, authTag } = await WebRatchet.encryptPayload(keyBytes, iv, plaintext, aad);
    expect(uint8ArrayToBase64(ciphertext)).toBe(fixtures.ts_aead.ciphertext_b64);
    expect(uint8ArrayToBase64(authTag)).toBe(fixtures.ts_aead.auth_tag_b64);
  });

  it('Envelope JSON Wire Deserialization matches Go protocol wire format', () => {
    const parsed = JSON.parse(fixtures.envelope_wire.wire_json);
    expect(parsed.id).toBe(fixtures.envelope_wire.envelope.id);
    expect(parsed.recipient_id).toBe(fixtures.envelope_wire.envelope.recipient_id);
    expect(parsed.sender_id).toBe(fixtures.envelope_wire.envelope.sender_id);
    expect(parsed.iv).toBe(fixtures.envelope_wire.envelope.iv);
    expect(parsed.ciphertext).toBe(fixtures.envelope_wire.envelope.ciphertext);
    expect(parsed.auth_tag).toBe(fixtures.envelope_wire.envelope.auth_tag);
  });

  it('End-to-End Pairwise Encrypt and Decrypt in Web', async () => {
    const plain = 'Confidential test message from Alice to Bob in browser';
    const env = await WebRatchet.encryptMessage('student_alice', '@bob', plain, 5000);
    const decrypted = await WebRatchet.decryptMessage(env);
    expect(decrypted).toBe(plain);
  });
});
