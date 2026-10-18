import React, { useState, useEffect } from 'react';
import { Terminal, Shield, Flame, Users, Lock, Zap } from 'lucide-react';
import { PresenceBadge } from './components/PresenceBadge.tsx';
import { ChatFeed, DisplayMessage } from './components/ChatFeed.tsx';
import { DevicePairing } from './components/DevicePairing.tsx';
import { WebKeyStore } from './crypto/keys.ts';
import { WebRatchet } from './crypto/ratchet.ts';
import { useWebSocket } from './hooks/useWebSocket.ts';

export const App: React.FC = () => {
  const [currentUser] = useState('student_alice');
  const [currentContact, setCurrentContact] = useState('@bob');
  const [locationEnabled, setLocationEnabled] = useState(false);
  const [zoneDisplayName, setZoneDisplayName] = useState('Library 2nd Floor - Silent Study');
  const [buildingName, setBuildingName] = useState<string | undefined>('Main Library');
  const [isPairingOpen, setIsPairingOpen] = useState(false);
  const [messages, setMessages] = useState<DisplayMessage[]>([]);
  const [token, setToken] = useState<string | null>(null);

  const keyStore = React.useMemo(() => new WebKeyStore(), []);

  // Initialize simulated session token for demonstration
  useEffect(() => {
    // Generate simulated client token for WebSocket connection
    const mockToken = btoa(JSON.stringify({
      device_id: 'dev-web-client',
      user_handle: currentUser,
      platform: 'web',
      iat: Math.floor(Date.now() / 1000),
      exp: Math.floor(Date.now() / 1000) + 86400,
    }));
    setToken(mockToken);

    // Initial demo messages
    const now = Date.now();
    setMessages([
      {
        id: 'msg-init-1',
        sender: '@bob',
        recipient: currentUser,
        plaintext: 'Hey Alice! Got the encrypted study notes for Lab 4?',
        timestampMS: now - 3000,
        ttlMS: 20000,
      },
    ]);
  }, [currentUser]);

  const { isConnected, sendEnvelope, ackRead, updatePresence, incomingEnvelope, burnedMessageId } =
    useWebSocket(token);

  // Handle incoming WebSocket envelopes
  useEffect(() => {
    if (!incomingEnvelope) return;
    // Decrypt envelope using WebCrypto
    const sessionSecret = 'CampusNet_Ratchet_Shared_Demo_Secret';
    WebRatchet.decryptMessage(incomingEnvelope, sessionSecret)
      .then(plaintext => {
        setMessages(prev => [
          ...prev,
          {
            id: incomingEnvelope.id,
            sender: incomingEnvelope.sender_id,
            recipient: incomingEnvelope.recipient_id,
            plaintext: plaintext,
            timestampMS: incomingEnvelope.timestamp_ms,
            ttlMS: incomingEnvelope.ttl_ms,
          },
        ]);
      })
      .catch(err => {
        console.error('Decryption error:', err);
      });
  }, [incomingEnvelope]);

  // Handle burn notifications
  useEffect(() => {
    if (!burnedMessageId) return;
    setMessages(prev => prev.filter(m => m.id !== burnedMessageId));
  }, [burnedMessageId]);

  const handleSendMessage = async (text: string, ttlMS: number) => {
    const sessionSecret = 'CampusNet_Ratchet_Shared_Demo_Secret';
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

  const contacts = ['@bob', '@carol', '@study_group', '@robotics_lab'];

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
                    onClick={() => setCurrentContact(c)}
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
                    <Lock className="w-3 h-3 text-gray-500" />
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
              messages={messages}
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
      />
    </div>
  );
};
