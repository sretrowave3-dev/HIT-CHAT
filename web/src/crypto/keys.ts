/**
 * CampusNet WebCrypto Key Store
 * Zero-Knowledge Invariant: Private keys are generated as NON-EXPORTABLE (extractable: false)
 * and persisted exclusively within browser IndexedDB hardware/software isolation.
 */

const DB_NAME = 'CampusNet_Security_Vault';
const DB_VERSION = 1;
const STORE_NAME = 'crypto_keys';

export interface StoredKeyPair {
  id: string;
  publicKeyRaw: string; // Base64
  privateKey: CryptoKey; // Non-exportable CryptoKey
  createdAt: number;
}

export class WebKeyStore {
  private dbPromise: Promise<IDBDatabase>;

  constructor() {
    this.dbPromise = new Promise((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, DB_VERSION);
      req.onupgradeneeded = () => {
        const db = req.result;
        if (!db.objectStoreNames.contains(STORE_NAME)) {
          db.createObjectStore(STORE_NAME, { keyPath: 'id' });
        }
      };
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
  }

  /**
   * Generates a non-exportable ECDH keypair using P-256 / X25519 in WebCrypto.
   */
  async generateDeviceIdentityKey(): Promise<{ id: string; publicKeyBase64: string }> {
    const keyPair = await window.crypto.subtle.generateKey(
      {
        name: 'ECDH',
        namedCurve: 'P-256',
      },
      false, // EXTRACTABLE: FALSE (Cannot be extracted by scripts or disk dumps)
      ['deriveKey', 'deriveBits']
    );

    const pubExported = await window.crypto.subtle.exportKey('raw', keyPair.publicKey);
    const pubB64 = btoa(String.fromCharCode(...new Uint8Array(pubExported)));
    const keyId = 'device_identity_key';

    const db = await this.dbPromise;
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readwrite');
      const store = tx.objectStore(STORE_NAME);
      store.put({
        id: keyId,
        publicKeyRaw: pubB64,
        privateKey: keyPair.privateKey,
        createdAt: Date.now(),
      });
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    });

    return { id: keyId, publicKeyBase64: pubB64 };
  }

  /**
   * Retrieves non-exportable private key reference from IndexedDB.
   */
  async getPrivateKey(keyId: string): Promise<CryptoKey | null> {
    const db = await this.dbPromise;
    return new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readonly');
      const store = tx.objectStore(STORE_NAME);
      const req = store.get(keyId);
      req.onsuccess = () => {
        if (req.result) {
          resolve(req.result.privateKey);
        } else {
          resolve(null);
        }
      };
      req.onerror = () => reject(req.error);
    });
  }

  /**
   * Wipes all private keys from IndexedDB (Emergency Shredder).
   */
  async purgeAllKeys(): Promise<void> {
    const db = await this.dbPromise;
    return new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readwrite');
      const store = tx.objectStore(STORE_NAME);
      const req = store.clear();
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  }
}
