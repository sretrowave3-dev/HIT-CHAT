import React, { useState, useEffect } from 'react';
import { Terminal, Shield, Flame, Users, Lock, Zap } from 'lucide-react';
import { PresenceBadge } from './components/PresenceBadge.tsx';
import { ChatFeed, DisplayMessage } from './components/ChatFeed.tsx';
import { DevicePairing } from './components/DevicePairing.tsx';
import { WebKeyStore } from './crypto/keys.ts';
import { WebRatchet } from './crypto/ratchet.ts';
import { useWebSocket } from './hooks/useWebSocket.ts';

export const App: React.FC = () => {
  const [currentUser] = useState('alice');
  const [contacts, setContacts] = useState<string[]>(['@bob', '@carol', '@study_group', '@robotics_lab']);
  const [currentContact, setCurrentContact] = useState('@bob');
  const [locationEnabled, setLocationEnabled] = useState(false);
  const [zoneDisplayName, setZoneDisplayName] = useState('Library 2nd Floor - Silent Study');
  const [buildingName, setBuildingName] = useState<string | undefined>('Main Library');
  const [isPairingOpen, setIsPairingOpen] = useState(false);
  const [messages, setMessages] = useState<DisplayMessage[]>([]);
  const [token, setToken] = useState<string | null>(null);
  const [unreadCounts, setUnreadCounts] = useState<Record<string, number>>({});

  const keyStore = React.useMemo(() => new WebKeyStore(), []);

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
        sender: '@bob',
        recipient: currentUser,
        plaintext: 'Hey Alice! Got the encrypted study notes for Lab 4?',
        timestampMS: now - 3000,
        ttlMS: 30000,
      },
    ]);
  }, [currentUser, keyStore]);

  const { isConnected, sendEnvelope, ackRead, updatePresence, incomingEnvelope, burnedMessageId } =
    useWebSocket(token);

  // Handle incoming WebSocket envelopes
  useEffect(() => {
    if (!incomingEnvelope) return;

    const processEnvelope = async () => {
      let plaintext = '';
      try {
        const fallbackSecret = 'CampusNet_Ratchet_Shared_Demo_Secret';
        plaintext = await WebRatchet.decryptWithPairwiseFallback(incomingEnvelope, fallbackSecret);
      } catch {
        // Fallback for plain text or UTF-8 string
        try {
          const decoded = atob(incomingEnvelope.ciphertext);
          // Only use decoded if it's printable characters (not binary ciphertext bytes)
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
        if (!prev.includes(senderTag)) {
          return [senderTag, ...prev];
        }
        return prev;
      });

      // Track unread if not currently looking at this sender
      if (currentContact !== senderTag) {
        setUnreadCounts(prev => ({
          ...prev,
          [senderTag]: (prev[senderTag] || 0) + 1,
        }));
      }

      setMessages(prev => [
        ...prev,
        {
          id: incomingEnvelope.id,
          sender: senderTag,
          recipient: currentUser,
          plaintext: plaintext || '[Encrypted Ephemeral Message]',
          timestampMS: incomingEnvelope.timestamp_ms || Date.now(),
          ttlMS: incomingEnvelope.ttl_ms || 15000,
        },
      ]);
    };

    processEnvelope();
  }, [incomingEnvelope, currentUser]);

  // Handle burn notifications
  useEffect(() => {
    if (!burnedMessageId) return;
    setMessages(prev => prev.filter(m => m.id !== burnedMessageId));
  }, [burnedMessageId]);

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
    // Animate burning state
    setMessages(prev =>
      prev.map(m => (m.id === id ? { ...m, isBurning: true } : m))
    );
    // Send ACK_READ to trigger zero-fill on backend
    ackRead(id);
    setTimeout(() => {
      setMessages(prev => prev.filter(m => m.id !== id));
    }, 600);
  };

  const handleTogglePrivacy = () => {
    const nextState = !locationEnabled;
    setLocationEnabled(nextState);
    if (nextState) {
      setZoneDisplayName('Robotics Lab 4 (Zone: ENG_LAB_4)');
      setBuildingName('Turing Engineering Complex');
      updatePresence({
        bssidHash: '00:14:22:67:89:ab',
        locationEnabled: true,
      });
    } else {
      setZoneDisplayName('Privacy Protected (Location Concealed)');
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
    alert('Security Alert: All ephemeral private keys and message buffers shredded!');
  };

  return (
    <div className="flex flex-col h-screen bg-cyber-900 text-gray-100">
      {/* Top Navbar */}
      <header className="h-16 px-6 bg-cyber-800 border-b border-cyber-700 flex items-center justify-between shadow-md">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-emerald-500/20 text-emerald-400 rounded-xl border border-emerald-500/30">
            <Zap className="w-5 h-5 fill-emerald-400" />
          </div>
          <div>
            <h1 className="text-base font-extrabold tracking-wide flex items-center gap-2">
              <span>CAMPUSNET</span>
              <span className="text-[10px] uppercase font-mono px-2 py-0.5 bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 rounded">
                E2EE Core
              </span>
            </h1>
            <p className="text-[11px] text-gray-400">Zero-Knowledge Ephemeral Relay</p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          {/* Connection status */}
          <div className="flex items-center gap-2 px-3 py-1 bg-cyber-900/80 border border-gray-700 rounded-lg text-xs font-mono">
            <span
              className={`w-2 h-2 rounded-full ${
                isConnected ? 'bg-emerald-400 shadow-sm shadow-emerald-400' : 'bg-amber-400'
              }`}
            />
            <span className="text-gray-300">@{currentUser}</span>
          </div>

          {/* Pair CLI Button */}
          <button
            onClick={() => setIsPairingOpen(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-cyber-700 hover:bg-cyber-600 border border-gray-600 rounded-lg text-xs font-semibold text-gray-200 transition"
          >
            <Terminal className="w-3.5 h-3.5 text-emerald-400" />
            <span>Pair CLI</span>
          </button>

          {/* Emergency Shred Button */}
          <button
            onClick={handleEmergencyShred}
            title="Wipe all keys and message memory"
            className="flex items-center gap-1.5 px-3 py-1.5 bg-red-950/40 hover:bg-red-900/60 border border-red-500/40 rounded-lg text-xs font-semibold text-red-300 transition"
          >
            <Flame className="w-3.5 h-3.5 text-red-400" />
            <span>Emergency Shred</span>
          </button>
        </div>
      </header>

      {/* Main Content Layout */}
      <div className="flex-1 flex flex-col p-6 max-w-7xl w-full mx-auto gap-4 overflow-hidden">
        {/* Presence & Privacy Switch Bar */}
        <PresenceBadge
          locationEnabled={locationEnabled}
          zoneDisplayName={zoneDisplayName}
          buildingName={buildingName}
          onTogglePrivacy={handleTogglePrivacy}
        />

        {/* Messaging Layout */}
        <div className="flex-1 grid grid-cols-1 md:grid-cols-4 gap-4 overflow-hidden">
          {/* Left Contacts Sidebar */}
          <div className="bg-cyber-800/60 border border-cyber-700 rounded-2xl p-4 flex flex-col gap-2 shadow-xl">
            <div className="flex items-center gap-2 px-2 py-2 text-xs font-semibold text-gray-400 uppercase tracking-wider">
              <Users className="w-4 h-4 text-emerald-400" />
              <span>Campus Peers</span>
            </div>

            <div className="space-y-1.5 flex-1 overflow-y-auto">
              {contacts.map(c => {
                const isActive = c === currentContact;
                return (
                  <button
                    key={c}
                    onClick={() => {
                      setCurrentContact(c);
                      setUnreadCounts(prev => ({ ...prev, [c]: 0 }));
                    }}
                    className={`w-full text-left px-3 py-2.5 rounded-xl text-sm font-medium transition flex items-center justify-between ${
                      isActive
                        ? 'bg-emerald-500/10 text-emerald-300 border border-emerald-500/30'
                        : 'text-gray-300 hover:bg-cyber-700/50 hover:text-white'
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <div className="w-2 h-2 rounded-full bg-emerald-400" />
                      <span>{c}</span>
                    </div>
                    <div className="flex items-center gap-1.5">
                      {(unreadCounts[c] || 0) > 0 && (
                        <span className="px-1.5 py-0.5 text-[10px] font-bold bg-emerald-500 text-black rounded-full animate-pulse">
                          {unreadCounts[c]}
                        </span>
                      )}
                      <Lock className="w-3 h-3 text-gray-500" />
                    </div>
                  </button>
                );
              })}
            </div>

            <div className="p-3 bg-cyber-900/60 rounded-xl border border-gray-800 text-[11px] text-gray-400">
              <div className="flex items-center gap-1.5 font-semibold text-gray-300 mb-1">
                <Shield className="w-3.5 h-3.5 text-emerald-400" />
                <span>Zero-Trace Guarantees</span>
              </div>
              Messages reside strictly in RAM with millisecond TTL eviction. No unencrypted server persistence.
            </div>
          </div>

          {/* Right Chat Feed */}
          <div className="md:col-span-3 h-full">
            <ChatFeed
              currentContact={currentContact}
              currentUser={currentUser}
              messages={messages.filter(m =>
                m.recipient === currentContact ||
                m.sender === currentContact ||
                m.recipient === currentContact.replace('@', '') ||
                m.sender === currentContact.replace('@', '')
              )}
              onSendMessage={handleSendMessage}
              onBurnMessage={handleBurnMessage}
            />
          </div>
        </div>
      </div>

      {/* Terminal Device Pairing Modal */}
      <DevicePairing
        userHandle={currentUser}
        isOpen={isPairingOpen}
        onClose={() => setIsPairingOpen(false)}
        token={token}
      />
    </div>
  );
};
