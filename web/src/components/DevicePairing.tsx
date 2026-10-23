import React, { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { IconClose, SparkleMotif, ChevronMotif } from './PosterMotifs.tsx';

interface DevicePairingProps {
  userHandle: string;
  isOpen: boolean;
  onClose: () => void;
  token?: string | null;
}

export const DevicePairing: React.FC<DevicePairingProps> = ({
  userHandle,
  isOpen,
  onClose,
  token,
}) => {
  const [userCode, setUserCode] = useState('');
  const [status, setStatus] = useState<'idle' | 'authorizing' | 'success' | 'error'>('idle');
  const [errorMessage, setErrorMessage] = useState('');
  const [pendingDevices, setPendingDevices] = useState<any[]>([]);

  // Check for pending terminal devices on open
  useEffect(() => {
    if (!isOpen) return;
    const fetchPending = async () => {
      try {
        const res = await fetch('/api/v1/auth/device/pending', {
          headers: token ? { 'Authorization': `Bearer ${token}` } : {},
        });
        if (res.ok) {
          const data = await res.json();
          if (data.pending_devices && data.pending_devices.length > 0) {
            setPendingDevices(data.pending_devices);
            setUserCode(data.pending_devices[0].user_code);
          } else {
            setPendingDevices([]);
          }
        }
      } catch {
        // Ignore network errors on background poll
      }
    };

    fetchPending();
    const interval = setInterval(fetchPending, 3000);
    return () => clearInterval(interval);
  }, [isOpen, token]);

  const handleAutoAuthorize = async (code?: string) => {
    setStatus('authorizing');
    setErrorMessage('');

    try {
      const res = await fetch('/api/v1/auth/device/auto', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          user_code: (code || userCode || '').trim().toUpperCase(),
          user_handle: userHandle,
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to authorize terminal');
      }

      setStatus('success');
      setTimeout(() => {
        onClose();
        setStatus('idle');
        setUserCode('');
      }, 1500);
    } catch (err: any) {
      setStatus('error');
      setErrorMessage(err.message || 'Authorization failed');
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!userCode.trim()) return;
    await handleAutoAuthorize(userCode);
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-poster-ink/75 p-4">
          <motion.div
            initial={{ scale: 0.9, opacity: 0, y: 15 }}
            animate={{ scale: 1, opacity: 1, y: 0 }}
            exit={{ scale: 0.9, opacity: 0, y: 15 }}
            transition={{ type: 'spring', damping: 22, stiffness: 350 }}
            className="bg-poster-cream paper-surface border-2 border-poster-ink shadow-hard-lg w-full max-w-md p-6 relative rounded-[28px_8px_28px_8px] overflow-hidden"
          >
            {/* Halftone corner pattern */}
            <div className="absolute top-0 right-0 w-24 h-24 poster-halftone-pattern rounded-tr-[24px]" />

            {/* Close Button */}
            <motion.button
              whileTap={{ scale: 0.92 }}
              type="button"
              onClick={onClose}
              className="poster-btn absolute top-4 right-4 p-1.5 bg-white hover:bg-poster-red hover:text-white text-poster-ink rounded-[8px_2px_8px_2px]"
              title="CLOSE"
            >
              <IconClose size={16} />
            </motion.button>

            {/* Header */}
            <div className="mb-4 pb-3 border-b-2 border-poster-ink relative z-10">
              <div className="flex items-center gap-1.5 font-mono text-[10px] font-bold text-poster-deepred uppercase tracking-widest">
                <SparkleMotif size={12} color="#D7191F" />
                <span>HARDWARE PROTOCOL / RFC 8628</span>
              </div>
              <h3 className="font-bungee text-2xl text-poster-ink tracking-wide mt-1">
                PAIR CLI TERMINAL
              </h3>
            </div>

            {/* Pending Devices Alert */}
            {pendingDevices.length > 0 && (
              <div className="mb-4 p-3 bg-poster-yellow border-2 border-poster-ink rounded-[16px_4px_16px_4px] shadow-hard-sm relative z-10">
                <div className="flex items-center justify-between mb-1.5">
                  <span className="font-mono text-xs font-bold text-poster-ink uppercase">
                    CLI PAIRING REQUEST DETECTED:
                  </span>
                  <span className="font-mono text-[10px] font-bold bg-poster-green text-poster-cream px-1.5 py-0.5 border border-poster-ink uppercase">
                    ACTIVE
                  </span>
                </div>
                <div className="flex items-center justify-between gap-3 mt-2">
                  <span className="font-mono text-base font-bold tracking-widest text-poster-ink bg-white px-2.5 py-1 border border-poster-ink rounded-[6px_2px_6px_2px]">
                    {pendingDevices[0].user_code}
                  </span>
                  <motion.button
                    whileTap={{ scale: 0.95 }}
                    type="button"
                    onClick={() => handleAutoAuthorize(pendingDevices[0].user_code)}
                    disabled={status === 'authorizing'}
                    className="poster-btn px-3 py-1.5 bg-poster-green text-poster-cream font-bungee text-xs uppercase rounded-[8px_2px_8px_2px]"
                  >
                    AUTHORIZE
                  </motion.button>
                </div>
              </div>
            )}

            {/* Pairing Code Manual Form */}
            <form onSubmit={handleSubmit} className="space-y-4 relative z-10">
              <div>
                <label
                  htmlFor="user-code-input"
                  className="block font-mono text-xs font-bold text-poster-ink uppercase mb-1"
                >
                  ENTER 8-CHAR USER CODE:
                </label>
                <input
                  id="user-code-input"
                  type="text"
                  maxLength={9}
                  placeholder="ABCD-EFGH"
                  value={userCode}
                  onChange={e => setUserCode(e.target.value.toUpperCase())}
                  className="w-full bg-white border-2 border-poster-ink text-poster-ink font-mono text-lg font-bold text-center tracking-widest px-3 py-2.5 shadow-hard-sm focus:outline-none focus:bg-poster-cream rounded-[16px_4px_16px_4px] placeholder:text-poster-ink/40 uppercase"
                />
              </div>

              {/* Status alerts */}
              {status === 'error' && (
                <div className="p-2.5 bg-poster-red/20 border-2 border-poster-red text-poster-red font-mono text-xs font-bold uppercase rounded-[8px_2px_8px_2px]">
                  {errorMessage}
                </div>
              )}

              {status === 'success' && (
                <div className="p-2.5 bg-poster-green/20 border-2 border-poster-green text-poster-green font-mono text-xs font-bold uppercase rounded-[8px_2px_8px_2px]">
                  CLI TERMINAL LINKED SUCCESSFULLY!
                </div>
              )}

              <div className="pt-2">
                <motion.button
                  whileTap={{ scale: 0.95 }}
                  type="submit"
                  disabled={status === 'authorizing' || !userCode.trim()}
                  className="poster-btn w-full px-4 py-3 bg-poster-mustard hover:bg-poster-yellow text-poster-ink font-bungee text-xs uppercase rounded-[16px_4px_16px_4px] disabled:opacity-50 flex items-center justify-center gap-2 cursor-pointer"
                >
                  <ChevronMotif size={12} />
                  <span>
                    {status === 'authorizing' ? 'LINKING KEYRING…' : 'CONFIRM LINK'}
                  </span>
                  <ChevronMotif size={12} className="rotate-180" />
                </motion.button>
              </div>
            </form>
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  );
};
