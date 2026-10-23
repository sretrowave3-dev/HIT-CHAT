import React, { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { motion } from 'framer-motion';
import { ChatFeed, DisplayMessage } from './components/ChatFeed.tsx';
import { DevicePairing } from './components/DevicePairing.tsx';
import { PrivacyInfoModal } from './components/PresenceBadge.tsx';
import { DiagnosticsModal } from './components/DiagnosticsModal.tsx';
import { WebKeyStore } from './crypto/keys.ts';
import { WebRatchet, WebEnvelope, canonicalHandle } from './crypto/ratchet.ts';
import { useWebSocket } from './hooks/useWebSocket.ts';
import {
  SparkleMotif,
  LotusMotif,
  ChevronMotif,
  IconPlus,
  IconLock,
  DraggableSticker,
} from './components/PosterMotifs.tsx';

export const App: React.FC = () => {
  const [currentUser] = useState('alice');
  const [contacts, setContacts] = useState<string[]>([
    '@student_guest',
    '@bob',
    '@carol',
    '@study_group',
    '@robotics_lab',
  ]);
  const [currentContact, setCurrentContact] = useState('@student_guest');
  const [newContactInput, setNewContactInput] = useState('');
  const [locationEnabled, setLocationEnabled] = useState(false);
  const [zoneDisplayName, setZoneDisplayName] = useState('Library 2nd Floor');
  const [buildingName, setBuildingName] = useState<string | undefined>('Main Library');
  const [isPairingOpen, setIsPairingOpen] = useState(false);
  const [isPrivacyInfoOpen, setIsPrivacyInfoOpen] = useState(false);
  const [messages, setMessages] = useState<DisplayMessage[]>([]);
  const [token, setToken] = useState<string | null>(null);
  const [unreadCounts, setUnreadCounts] = useState<Record<string, number>>({});
  const [isShutterActive, setIsShutterActive] = useState(false);

  const keyStore = useMemo(() => new WebKeyStore(), []);

  // Initialize authenticated session and register PreKeyBundle on backend
  useEffect(() => {
    async function initSession() {
      try {
        const res = await fetch('/api/v1/auth/session', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            user_handle: currentUser,
            platform: 'web',
          }),
        });
        if (res.ok) {
          const data = await res.json();
          setToken(data.access_token);

          // Register Pre-Key bundle for Alice on backend so CLI and peers can send E2EE messages to her
          try {
            const idKey = await keyStore.generateDeviceIdentityKey();
            await fetch('/api/v1/crypto/bundle', {
              method: 'POST',
              headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${data.access_token}`,
              },
              body: JSON.stringify({
                device_id: data.device_id,
                user_handle: currentUser,
                identity_key: idKey.publicKeyBase64,
                signed_prekey: idKey.publicKeyBase64,
                signed_prekey_sig: 'web_verified_signature_p256',
                one_time_prekeys: [
                  btoa(`otk_${currentUser}_1_${Date.now()}`),
                  btoa(`otk_${currentUser}_2_${Date.now()}`),
                  btoa(`otk_${currentUser}_3_${Date.now()}`),
                ],
              }),
            });
            console.log(`[AUTH] PreKeyBundle registered for @${currentUser}`);
          } catch (kErr) {
            console.warn('Could not register prekey bundle:', kErr);
          }
        }
      } catch (err) {
        console.error('Failed to init authenticated session:', err);
      }
    }
    initSession();

    // Initial demo welcome message
    const now = Date.now();
    setMessages([
      {
        id: 'msg-init-1',
        sender: '@student_guest',
        recipient: currentUser,
        plaintext: 'Hello Alice. Encrypted study notes for Lab 4 are ready.',
        timestampMS: now - 3000,
        ttlMS: 30000,
        status: 'DELIVERED',
      },
    ]);
  }, [currentUser, keyStore]);

  const seenEnvelopeIds = useRef<Set<string>>(new Set(['msg-init-1']));
  const [lastAckedId, setLastAckedId] = useState<string>('none');
  const [isDiagnosticsOpen, setIsDiagnosticsOpen] = useState(false);

  const handleBurn = useCallback((messageId: string) => {
    setMessages(prev => prev.filter(m => m.id !== messageId));
  }, []);

  const handleAckDelivered = useCallback((messageId: string) => {
    setLastAckedId(messageId);
    setMessages(prev =>
      prev.map(m => (m.id === messageId && m.status !== 'READ' ? { ...m, status: 'DELIVERED' } : m))
    );
  }, []);

  const handleAckRead = useCallback((messageId: string) => {
    setLastAckedId(messageId);
    setMessages(prev =>
      prev.map(m => (m.id === messageId ? { ...m, status: 'READ' } : m))
    );
  }, []);

  const handleNack = useCallback((messageId: string, reason: string) => {
    console.warn(`[NACK received for message ${messageId}]: ${reason}`);
    setMessages(prev =>
      prev.map(m => (m.id === messageId ? { ...m, status: 'FAILED' } : m))
    );
  }, []);

  const [peerTypingMap, setPeerTypingMap] = useState<Record<string, boolean>>({});

  const handleTypingNotification = useCallback((senderId: string, isTyping: boolean) => {
    setPeerTypingMap(prev => ({ ...prev, [senderId]: isTyping }));
  }, []);

  const wsOptions = useMemo(
    () => ({
      onBurn: handleBurn,
      onAckDelivered: handleAckDelivered,
      onAckRead: handleAckRead,
      onNack: handleNack,
      onTyping: handleTypingNotification,
    }),
    [handleBurn, handleAckDelivered, handleAckRead, handleNack, handleTypingNotification]
  );

  const {
    isConnected,
    outboxLength,
    sendEnvelope,
    ackDelivered,
    ackRead,
    sendNack,
    sendTyping,
    updatePresence,
  } = useWebSocket(token, wsOptions);

  const handleIncomingEnvelope = useCallback(
    async (incomingEnvelope: WebEnvelope) => {
      // 1. DEDUPLICATE BEFORE DECRYPTION
      if (seenEnvelopeIds.current.has(incomingEnvelope.id)) {
        return;
      }
      seenEnvelopeIds.current.add(incomingEnvelope.id);

      const senderTag = incomingEnvelope.sender_id.startsWith('@')
        ? incomingEnvelope.sender_id
        : '@' + incomingEnvelope.sender_id;

      // Add sender to contacts if not present
      setContacts(prev => {
        const exists = prev.some(c => canonicalHandle(c) === canonicalHandle(senderTag));
        if (!exists) {
          return [senderTag, ...prev];
        }
        return prev;
      });

      let plaintext = '';
      let isDecryptError = false;

      try {
        plaintext = await WebRatchet.decryptMessage(incomingEnvelope);
        // Successful decrypt: notify sender with ACK_DELIVERED
        ackDelivered(incomingEnvelope.id);
      } catch (err: any) {
        console.warn('[Decryption failed for envelope]', incomingEnvelope.id, err);
        // FAILED DECRYPT:
        // 1. NEVER trigger OpAckRead!
        // 2. Send OpNack with error details to server!
        sendNack(incomingEnvelope.id, 'DECRYPT_FAILED: ' + (err?.message || 'Decryption error'));
        isDecryptError = true;
        plaintext = 'Cannot decrypt (tap to retry / re-key)';
      }

      // Track unread if not currently looking at this sender
      const isViewing = canonicalHandle(currentContact) === canonicalHandle(senderTag);
      if (!isViewing) {
        setUnreadCounts(prev => ({
          ...prev,
          [senderTag]: (prev[senderTag] || 0) + 1,
        }));
      } else if (!isDecryptError) {
        // Automatically send read receipt if actively viewing
        ackRead(incomingEnvelope.id);
      }

      setMessages(prev => {
        if (prev.some(m => m.id === incomingEnvelope.id)) {
          return prev;
        }
        return [
          ...prev,
          {
            id: incomingEnvelope.id,
            sender: senderTag,
            recipient: currentUser,
            plaintext: plaintext,
            timestampMS: incomingEnvelope.timestamp_ms || Date.now(),
            ttlMS: incomingEnvelope.ttl_ms || 15000,
            status: isDecryptError ? 'FAILED' : 'DELIVERED',
            isDecryptError: isDecryptError,
            rawEnvelope: incomingEnvelope,
          },
        ];
      });
    },
    [currentContact, currentUser, ackDelivered, ackRead, sendNack]
  );

  // Bind onEnvelope to websocket handler
  useEffect(() => {
    (wsOptions as any).onEnvelope = handleIncomingEnvelope;
  }, [wsOptions, handleIncomingEnvelope]);

  const handleRetryDecrypt = useCallback(async (msg: DisplayMessage) => {
    if (!msg.rawEnvelope) return;
    try {
      const decrypted = await WebRatchet.decryptMessage(msg.rawEnvelope);
      setMessages(prev =>
        prev.map(m =>
          m.id === msg.id
            ? { ...m, plaintext: decrypted, isDecryptError: false, status: 'DELIVERED' }
            : m
        )
      );
      ackDelivered(msg.id);
      ackRead(msg.id);
    } catch (err: any) {
      alert('Retry failed: ' + (err?.message || 'Decryption error'));
    }
  }, [ackDelivered, ackRead]);

  const handleAddContact = () => {
    const raw = newContactInput.trim();
    if (!raw) return;
    const tag = raw.startsWith('@') ? raw : '@' + raw;
    if (!contacts.some(c => canonicalHandle(c) === canonicalHandle(tag))) {
      setContacts(prev => [tag, ...prev]);
    }
    setCurrentContact(tag);
    setNewContactInput('');
  };

  const handleSendMessage = async (text: string, ttlMS: number) => {
    const sessionSecret = WebRatchet.getPairwiseSecret(currentUser, currentContact);
    const env = await WebRatchet.encryptMessage(
      currentUser,
      currentContact.replace('@', ''),
      text,
      ttlMS,
      sessionSecret
    );

    // Optimistically add to UI
    const newMessage: DisplayMessage = {
      id: env.id,
      sender: currentUser,
      recipient: currentContact,
      plaintext: text,
      timestampMS: env.timestamp_ms,
      ttlMS: env.ttl_ms,
      status: 'SENDING',
    };
    seenEnvelopeIds.current.add(env.id);
    setMessages(prev => [...prev, newMessage]);

    // Dispatch envelope over WebSocket
    sendEnvelope(env);
  };

  const handleBurnMessage = (id: string) => {
    setMessages(prev => prev.map(m => (m.id === id ? { ...m, isBurning: true } : m)));
    ackRead(id);
    setTimeout(() => {
      setMessages(prev => prev.filter(m => m.id !== id));
    }, 350);
  };

  const handleTogglePrivacy = () => {
    // Trigger shutter wipe animation
    setIsShutterActive(true);
    setTimeout(() => setIsShutterActive(false), 450);

    const nextState = !locationEnabled;
    setLocationEnabled(nextState);
    if (nextState) {
      setZoneDisplayName('Robotics Lab 4 (ENG_LAB_4)');
      setBuildingName('Engineering Complex');
      updatePresence({
        bssidHash: '00:14:22:67:89:ab',
        locationEnabled: true,
      });
    } else {
      setZoneDisplayName('Private Mode');
      setBuildingName(undefined);
      updatePresence({
        bssidHash: '',
        locationEnabled: false,
      });
    }
  };

  const handleEmergencyShred = async () => {
    await keyStore.purgeAllKeys();
    setMessages([]);
    alert('KEYS ZEROED / RAM PURGED');
  };

  return (
    <div className="relative min-h-screen bg-poster-red p-3 sm:p-5 flex flex-col justify-between overflow-x-hidden selection:bg-poster-mustard selection:text-poster-ink">
      {/* Global Grain Overlay (18% opacity, animated steps(6) jitter) */}
      <div className="poster-grain-overlay" aria-hidden="true" />

      {/* Halftone Dot Pattern pinned to bottom-right corner */}
      <div
        className="fixed bottom-0 right-0 w-48 h-48 poster-halftone-pattern pointer-events-none z-0"
        aria-hidden="true"
      />

      {/* Slow-Scrolling Marquee Ticker in Header Strip */}
      <div className="marquee-container bg-poster-ink text-poster-cream py-1 px-2 border-2 border-poster-ink mb-3 shadow-hard-sm z-10 cursor-default">
        <div className="marquee-content font-mono text-[11px] font-bold tracking-widest uppercase flex items-center gap-6">
          <span>â˜… BURN ON READ</span>
          <span>â€¢</span>
          <span>ZERO-KNOWLEDGE RAM ONLY</span>
          <span>â€¢</span>
          <span>END-TO-END ENCRYPTED</span>
          <span>â€¢</span>
          <span>NO PERSISTENT DISK LOGS</span>
          <span>â€¢</span>
          <span>PAIRWISE HKDF RATIO</span>
          <span>â€¢</span>
          <span>AUTOMATIC EPHEMERAL PURGE â˜…</span>
          <span className="mx-4">|</span>
          <span>â˜… BURN ON READ</span>
          <span>â€¢</span>
          <span>ZERO-KNOWLEDGE RAM ONLY</span>
          <span>â€¢</span>
          <span>END-TO-END ENCRYPTED</span>
          <span>â€¢</span>
          <span>NO PERSISTENT DISK LOGS</span>
          <span>â€¢</span>
          <span>PAIRWISE HKDF RATIO</span>
          <span>â€¢</span>
          <span>AUTOMATIC EPHEMERAL PURGE â˜…</span>
        </div>
      </div>

      {/* Corner Monospace Meta Labels */}
      <div className="hidden lg:flex justify-between items-center text-[10px] font-mono uppercase text-poster-cream font-bold px-2 pb-2 tracking-widest z-10 select-none">
        <span>POSTER NO. 01 / ED. 2026</span>
        <span>ZERO-KNOWLEDGE WIRE / VOLATILE RAM ONLY</span>
      </div>

      <div className="flex-1 flex flex-col max-w-7xl w-full mx-auto gap-4 z-10">
        {/* Slim Top Bar with Asymmetric Corners & Tactile Depth */}
        <header className="bg-poster-cream paper-surface border-2 border-poster-ink shadow-hard px-4 py-3 flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 rounded-[28px_8px_28px_8px] relative">
          {/* App Title & Meta Strip */}
          <div className="flex items-center gap-3">
            <LotusMotif size={26} />
            <h1 className="font-bungee text-2xl sm:text-3xl text-poster-ink tracking-wider poster-hard-title">
              CAMPUSNET
            </h1>
            <span className="hidden sm:inline font-mono text-[11px] font-bold text-poster-deepred tracking-widest uppercase border-l-2 border-poster-ink pl-3">
              NO 01 / 23.10.2026
            </span>
          </div>

          {/* User Handle & Status & Buttons */}
          <div className="flex flex-wrap items-center gap-2 sm:gap-3">
            {/* Presence status: sparkle + ZONE: PRIVATE with shutter effect */}
            <div className="relative overflow-hidden font-mono text-xs font-bold uppercase text-poster-ink bg-white border-2 border-poster-ink px-2.5 py-1.5 shadow-hard-sm rounded-[12px_4px_12px_4px] flex items-center gap-1.5">
              {/* Shutter Animation Overlay */}
              {isShutterActive && (
                <motion.div
                  initial={{ x: '-100%' }}
                  animate={{ x: '100%' }}
                  transition={{ duration: 0.4, ease: 'easeInOut' }}
                  className="absolute inset-0 bg-poster-yellow z-20 pointer-events-none"
                />
              )}

              <SparkleMotif size={13} color="#F7D63C" />
              <span>{locationEnabled ? `ZONE: ${zoneDisplayName}` : 'ZONE: PRIVATE'}</span>
              <button
                type="button"
                onClick={() => setIsPrivacyInfoOpen(true)}
                className="ml-1 text-poster-deepred hover:underline text-[11px]"
                title="PRIVACY DETAILS"
              >
                [?]
              </button>
            </div>

            {/* User Handle Box */}
            <div className="font-mono text-xs font-bold uppercase text-poster-ink bg-poster-yellow border-2 border-poster-ink px-2.5 py-1.5 shadow-hard-sm flex items-center gap-1.5 rounded-[12px_4px_12px_4px]">
              <span
                className={`w-2.5 h-2.5 rounded-full border border-poster-ink ${
                  isConnected ? 'bg-poster-green' : 'bg-poster-red'
                }`}
                title={isConnected ? 'CONNECTED' : 'DISCONNECTED'}
              />
              <span>@{currentUser.toUpperCase()}</span>
            </div>

            {/* Pair Device Button */}
            <motion.button
              whileTap={{ scale: 0.94 }}
              type="button"
              onClick={() => setIsPairingOpen(true)}
              className="poster-btn px-3 py-1.5 bg-white hover:bg-poster-yellow text-poster-ink font-bungee text-xs uppercase rounded-[12px_4px_12px_4px] cursor-pointer"
            >
              PAIR DEVICE
            </motion.button>

            {/* Diagnostics Button */}
            <motion.button
              whileTap={{ scale: 0.94 }}
              type="button"
              onClick={() => setIsDiagnosticsOpen(true)}
              className="poster-btn px-3 py-1.5 bg-white hover:bg-poster-yellow text-poster-ink font-bungee text-xs uppercase rounded-[12px_4px_12px_4px] cursor-pointer"
            >
              DIAGNOSTICS
            </motion.button>

            {/* Burn All Button */}
            <motion.button
              whileTap={{ scale: 0.94 }}
              type="button"
              onClick={handleEmergencyShred}
              title="PURGE ALL KEYS AND BUFFER"
              className="poster-btn px-3 py-1.5 bg-poster-mustard hover:bg-poster-yellow text-poster-ink font-bungee text-xs uppercase rounded-[12px_4px_12px_4px] cursor-pointer"
            >
              BURN ALL
            </motion.button>
          </div>
        </header>

        {/* Messaging Layout */}
        <div className="flex-1 grid grid-cols-1 md:grid-cols-4 gap-4 overflow-hidden min-h-[560px]">
          {/* Left Sidebar: Peers with Asymmetric Shape & Devanagari Watermark */}
          <aside className="bg-poster-cream paper-surface border-2 border-poster-ink shadow-hard p-4 flex flex-col gap-3 rounded-[28px_8px_28px_8px] relative overflow-hidden">
            {/* Halftone Dot Corner in Sidebar */}
            <div className="absolute bottom-0 left-0 w-28 h-28 poster-halftone-pattern rounded-bl-[24px]" />

            {/* Oversized Faded Devanagari Word Behind Sidebar Title */}
            <span
              className="absolute -top-3 right-1 font-yatra text-8xl text-poster-ink opacity-10 select-none pointer-events-none"
              aria-hidden="true"
            >
              à¤—à¤ªà¤¶à¤ª
            </span>

            {/* Sidebar Title */}
            <div className="flex items-center justify-between border-b-2 border-poster-ink pb-2 relative z-10">
              <div className="flex items-center gap-2">
                <ChevronMotif size={14} />
                <h2 className="font-bungee text-lg text-poster-ink tracking-wide">
                  PEERS
                </h2>
              </div>
              <span className="font-mono text-[11px] font-bold text-poster-deepred uppercase">
                {contacts.length} ACTIVE
              </span>
            </div>

            {/* Search/Add Input with Asymmetric Border */}
            <div className="flex gap-1.5 relative z-10">
              <input
                type="text"
                placeholder="ADD @PEERâ€¦"
                value={newContactInput}
                onChange={e => setNewContactInput(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter') handleAddContact();
                }}
                className="w-full bg-white border-2 border-poster-ink font-mono text-xs uppercase text-poster-ink px-3 py-2 shadow-hard-sm focus:outline-none focus:bg-poster-cream rounded-[14px_4px_14px_4px] placeholder:text-poster-ink/40"
              />
              <motion.button
                whileTap={{ scale: 0.93 }}
                type="button"
                onClick={handleAddContact}
                title="ADD PEER"
                className="poster-btn px-3 py-2 bg-poster-mustard hover:bg-poster-yellow text-poster-ink flex items-center justify-center shrink-0 rounded-[10px_3px_10px_3px] cursor-pointer"
              >
                <IconPlus size={14} />
              </motion.button>
            </div>

            {/* Peer Rows: Asymmetric Tiles with Selected Slap-On Rotation */}
            <div className="space-y-2 flex-1 overflow-y-auto pr-1 relative z-10">
              {contacts.map(c => {
                const isActive = canonicalHandle(c) === canonicalHandle(currentContact);
                const unread = unreadCounts[c] || 0;
                const initial = c.replace('@', '').charAt(0).toUpperCase();

                return (
                  <motion.button
                    key={c}
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.97 }}
                    type="button"
                    onClick={() => {
                      setCurrentContact(c);
                      setUnreadCounts(prev => ({ ...prev, [c]: 0 }));
                    }}
                    className={`w-full text-left p-2.5 border-2 border-poster-ink transition-all flex items-center justify-between rounded-[20px_6px_20px_6px] cursor-pointer ${
                      isActive
                        ? 'bg-poster-mustard shadow-hard font-bold text-poster-ink -rotate-1 scale-[1.01]'
                        : 'bg-white hover:bg-poster-yellow/40 text-poster-ink shadow-hard-sm'
                    }`}
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      {/* Square-ish Asymmetric Avatar with 2px ink border */}
                      <div className="w-8 h-8 rounded-[10px_3px_10px_3px] border-2 border-poster-ink bg-poster-yellow flex items-center justify-center font-bungee text-xs text-poster-ink shrink-0">
                        {initial}
                      </div>
                      <span className="font-mono text-xs uppercase truncate">
                        {c}
                      </span>
                    </div>

                    <div className="flex items-center gap-2 shrink-0">
                      {unread > 0 && (
                        <span className="font-mono text-[10px] font-bold bg-poster-red text-white border border-poster-ink px-1.5 py-0.5 rounded-[4px]">
                          {unread}
                        </span>
                      )}
                      <IconLock size={12} />
                    </div>
                  </motion.button>
                );
              })}
            </div>
          </aside>

          {/* Right Chat Feed (3 columns) */}
          <main className="md:col-span-3 h-full">
            <ChatFeed
              currentContact={currentContact}
              currentUser={currentUser}
              messages={messages.filter(m => {
                const activeCan = canonicalHandle(currentContact);
                return (
                  canonicalHandle(m.sender) === activeCan ||
                  canonicalHandle(m.recipient) === activeCan
                );
              })}
              isPeerTyping={!!peerTypingMap[currentContact] || !!peerTypingMap[canonicalHandle(currentContact)]}
              onSendMessage={handleSendMessage}
              onBurnMessage={handleBurnMessage}
              onRetryDecrypt={handleRetryDecrypt}
              onTyping={(isTyping) => sendTyping(currentContact.replace('@', ''), isTyping)}
            />
          </main>
        </div>
      </div>

      {/* Draggable Fun Stickers Layer (Can be dragged around with spring physics) */}
      <div className="fixed bottom-4 left-6 z-30 pointer-events-auto hidden sm:flex items-center gap-3">
        <DraggableSticker motif="chilli" label="à¤¤à¥€à¤–à¤¾" initialRotate={-8} />
        <DraggableSticker motif="charm" label="à¤¨à¤œà¤¼à¤° à¤¬à¤Ÿà¥à¤Ÿà¥‚" initialRotate={6} />
        <DraggableSticker motif="lotus" label="à¤—à¥à¤«à¥à¤¤à¤—à¥‚" initialRotate={-4} />
      </div>

      {/* Bottom Corner Meta Labels */}
      <div className="hidden lg:flex justify-between items-center text-[10px] font-mono uppercase text-poster-cream font-bold px-2 pt-2 tracking-widest z-10 select-none">
        <span>CAMPUSNET WORKSHOP / MUMBAI & NEW DELHI</span>
        <span>CONTEMPORARY RISOGRAPH ZINE SYSTEM</span>
      </div>

      {/* Terminal Device Pairing Modal */}
      <DevicePairing
        userHandle={currentUser}
        isOpen={isPairingOpen}
        onClose={() => setIsPairingOpen(false)}
        token={token}
      />

      {/* Location Privacy Specification Modal */}
      <PrivacyInfoModal
        isOpen={isPrivacyInfoOpen}
        onClose={() => setIsPrivacyInfoOpen(false)}
        locationEnabled={locationEnabled}
        zoneDisplayName={zoneDisplayName}
        buildingName={buildingName}
        onTogglePrivacy={handleTogglePrivacy}
      />

      {/* Zero-Knowledge Diagnostic Modal */}
      <DiagnosticsModal
        isOpen={isDiagnosticsOpen}
        onClose={() => setIsDiagnosticsOpen(false)}
        isConnected={isConnected}
        currentUser={currentUser}
        currentContact={currentContact}
        lastAckedId={lastAckedId}
        outboxLength={outboxLength}
      />
    </div>
  );
};
