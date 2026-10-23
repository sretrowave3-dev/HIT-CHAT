import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { ChatFeed, DisplayMessage } from './components/ChatFeed.tsx';
import { DevicePairing } from './components/DevicePairing.tsx';
import { PrivacyInfoModal } from './components/PresenceBadge.tsx';
import { WebKeyStore } from './crypto/keys.ts';
import { WebRatchet, WebEnvelope, canonicalHandle } from './crypto/ratchet.ts';
import { useWebSocket } from './hooks/useWebSocket.ts';
import { SparkleMotif, IconPlus, IconLock } from './components/PosterMotifs.tsx';

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

    // Initial demo welcome message (clean plain English)
    const now = Date.now();
    setMessages([
      {
        id: 'msg-init-1',
        sender: '@student_guest',
        recipient: currentUser,
        plaintext: 'Hello Alice. Encrypted study notes for Lab 4 are ready.',
        timestampMS: now - 3000,
        ttlMS: 30000,
      },
    ]);
  }, [currentUser, keyStore]);

  const handleIncomingEnvelope = useCallback(async (incomingEnvelope: WebEnvelope) => {
    let plaintext = '';
    try {
      const fallbackSecret = 'CampusNet_Ratchet_Shared_Demo_Secret';
      plaintext = await WebRatchet.decryptWithPairwiseFallback(incomingEnvelope, fallbackSecret);
    } catch {
      try {
        const decoded = atob(incomingEnvelope.ciphertext);
        if (/^[\x20-\x7E\s\u00A0-\uFFFF]*$/.test(decoded) && decoded.length > 0) {
          plaintext = decoded;
        } else {
          plaintext = '[Encrypted Ephemeral Message]';
        }
      } catch {
        plaintext = incomingEnvelope.ciphertext;
      }
    }

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

    // Track unread if not currently looking at this sender
    if (canonicalHandle(currentContact) !== canonicalHandle(senderTag)) {
      setUnreadCounts(prev => ({
        ...prev,
        [senderTag]: (prev[senderTag] || 0) + 1,
      }));
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
          plaintext: plaintext || '[Encrypted Ephemeral Message]',
          timestampMS: incomingEnvelope.timestamp_ms || Date.now(),
          ttlMS: incomingEnvelope.ttl_ms || 15000,
        },
      ];
    });
  }, [currentContact, currentUser]);

  const handleBurn = useCallback((messageId: string) => {
    setMessages(prev => prev.filter(m => m.id !== messageId));
  }, []);

  const wsOptions = useMemo(() => ({
    onEnvelope: handleIncomingEnvelope,
    onBurn: handleBurn,
  }), [handleIncomingEnvelope, handleBurn]);

  const { isConnected, sendEnvelope, ackRead, updatePresence } =
    useWebSocket(token, wsOptions);

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
    };
    setMessages(prev => [...prev, newMessage]);

    // Dispatch envelope over WebSocket
    sendEnvelope(env);
  };

  const handleBurnMessage = (id: string) => {
    setMessages(prev =>
      prev.map(m => (m.id === id ? { ...m, isBurning: true } : m))
    );
    ackRead(id);
    setTimeout(() => {
      setMessages(prev => prev.filter(m => m.id !== id));
    }, 300);
  };

  const handleTogglePrivacy = () => {
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
    <div className="relative min-h-screen bg-poster-red p-3 sm:p-5 flex flex-col justify-between overflow-x-hidden">
      {/* Global Grain Overlay (~8% opacity) */}
      <div className="poster-grain-overlay" aria-hidden="true" />

      {/* Halftone Dot Pattern pinned to ONE corner only (bottom-right) */}
      <div className="poster-corner-halftone" aria-hidden="true" />

      {/* Corner Monospace Meta Labels */}
      <div className="hidden lg:flex justify-between items-center text-[10px] font-mono uppercase text-poster-cream font-bold px-2 pb-2 tracking-widest z-10 select-none">
        <span>POSTER NO. 01 / ED. 2026</span>
        <span>ZERO-KNOWLEDGE WIRE / VOLATILE RAM ONLY</span>
      </div>

      <div className="flex-1 flex flex-col max-w-7xl w-full mx-auto gap-4 z-10">
        {/* Slim Top Bar with 2px Ink Border */}
        <header className="bg-poster-cream border-2 border-poster-ink shadow-hard px-4 py-2.5 flex items-center justify-between gap-3 rounded-none">
          {/* App Title & Meta Strip */}
          <div className="flex items-center gap-3">
            <h1 className="font-bungee text-xl sm:text-2xl text-poster-ink tracking-wider">
              CAMPUSNET
            </h1>
            <span className="hidden sm:inline font-mono text-[11px] font-bold text-poster-deepred tracking-widest uppercase border-l-2 border-poster-ink pl-3">
              NO 01 / 23.10.2026
            </span>
          </div>

          {/* User Handle & Status & Buttons */}
          <div className="flex items-center gap-2 sm:gap-3">
            {/* Presence status: sparkle + ZONE: PRIVATE */}
            <div className="hidden md:flex items-center gap-1.5 font-mono text-xs font-bold uppercase text-poster-ink bg-white border-2 border-poster-ink px-2.5 py-1 shadow-hard-sm">
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
            <div className="font-mono text-xs font-bold uppercase text-poster-ink bg-poster-yellow border-2 border-poster-ink px-2.5 py-1 shadow-hard-sm flex items-center gap-1.5">
              <span
                className={`w-2 h-2 border border-poster-ink ${
                  isConnected ? 'bg-poster-green' : 'bg-poster-red'
                }`}
                title={isConnected ? 'CONNECTED' : 'DISCONNECTED'}
              />
              <span>@{currentUser.toUpperCase()}</span>
            </div>

            {/* Pair Device Button */}
            <button
              type="button"
              onClick={() => setIsPairingOpen(true)}
              className="poster-btn px-3 py-1.5 bg-white hover:bg-poster-yellow text-poster-ink font-bungee text-xs uppercase"
            >
              PAIR DEVICE
            </button>

            {/* Burn All Button */}
            <button
              type="button"
              onClick={handleEmergencyShred}
              title="PURGE ALL KEYS AND BUFFER"
              className="poster-btn px-3 py-1.5 bg-poster-mustard hover:bg-poster-yellow text-poster-ink font-bungee text-xs uppercase"
            >
              BURN ALL
            </button>
          </div>
        </header>

        {/* Messaging Layout */}
        <div className="flex-1 grid grid-cols-1 md:grid-cols-4 gap-4 overflow-hidden min-h-[550px]">
          {/* Left Sidebar: Peers */}
          <aside className="bg-poster-cream border-2 border-poster-ink shadow-hard p-4 flex flex-col gap-3 rounded-none relative overflow-hidden">
            {/* Oversized Faded Devanagari Word Behind Sidebar Title Only */}
            <span
              className="absolute -top-3 right-1 font-yatra text-7xl text-poster-ink opacity-10 select-none pointer-events-none"
              aria-hidden="true"
            >
              गपशप
            </span>

            {/* Sidebar Title */}
            <div className="flex items-center justify-between border-b-2 border-poster-ink pb-2 relative z-10">
              <h2 className="font-bungee text-lg text-poster-ink tracking-wide">
                PEERS
              </h2>
              <span className="font-mono text-[11px] font-bold text-poster-deepred uppercase">
                {contacts.length} ACTIVE
              </span>
            </div>

            {/* Search/Add Input: Plain Bordered Rectangle */}
            <div className="flex gap-1.5 relative z-10">
              <input
                type="text"
                placeholder="ADD @PEER…"
                value={newContactInput}
                onChange={e => setNewContactInput(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter') handleAddContact();
                }}
                className="w-full bg-white border-2 border-poster-ink font-mono text-xs uppercase text-poster-ink px-3 py-2 shadow-hard-sm focus:outline-none focus:bg-poster-cream rounded-none placeholder:text-poster-ink/40"
              />
              <button
                type="button"
                onClick={handleAddContact}
                title="ADD PEER"
                className="poster-btn px-3 py-2 bg-poster-mustard hover:bg-poster-yellow text-poster-ink flex items-center justify-center shrink-0"
              >
                <IconPlus size={14} />
              </button>
            </div>

            {/* Peer Rows: Flat Rectangles */}
            <div className="space-y-2 flex-1 overflow-y-auto pr-1 relative z-10">
              {contacts.map(c => {
                const isActive = canonicalHandle(c) === canonicalHandle(currentContact);
                const unread = unreadCounts[c] || 0;
                const initial = c.replace('@', '').charAt(0).toUpperCase();

                return (
                  <button
                    key={c}
                    type="button"
                    onClick={() => {
                      setCurrentContact(c);
                      setUnreadCounts(prev => ({ ...prev, [c]: 0 }));
                    }}
                    className={`w-full text-left p-2.5 border-2 border-poster-ink transition-all flex items-center justify-between rounded-none ${
                      isActive
                        ? 'bg-poster-mustard shadow-hard font-bold text-poster-ink'
                        : 'bg-white hover:bg-poster-yellow/40 text-poster-ink shadow-hard-sm'
                    }`}
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      {/* Square Avatar with 2px ink border (NO circular avatars!) */}
                      <div className="w-8 h-8 rounded-none border-2 border-poster-ink bg-poster-yellow flex items-center justify-center font-bungee text-xs text-poster-ink shrink-0">
                        {initial}
                      </div>
                      <span className="font-mono text-xs uppercase truncate">
                        {c}
                      </span>
                    </div>

                    <div className="flex items-center gap-2 shrink-0">
                      {unread > 0 && (
                        <span className="font-mono text-[10px] font-bold bg-poster-red text-white border border-poster-ink px-1.5 py-0.5">
                          {unread}
                        </span>
                      )}
                      <IconLock size={12} />
                    </div>
                  </button>
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
              onSendMessage={handleSendMessage}
              onBurnMessage={handleBurnMessage}
            />
          </main>
        </div>
      </div>

      {/* Bottom Corner Labels */}
      <div className="hidden lg:flex justify-between items-center text-[10px] font-mono uppercase text-poster-cream font-bold px-2 pt-2 tracking-widest z-10 select-none">
        <span>CAMPUSNET WORKSHOP / MUMBAI & NEW DELHI</span>
        <span>100% RETRO PRINT POSTER SYSTEM</span>
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
    </div>
  );
};
