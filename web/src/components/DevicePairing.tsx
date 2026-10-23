import React, { useState, useEffect } from 'react';
import { IconClose } from './PosterMotifs.tsx';

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

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!userCode.trim()) return;
    await handleAutoAuthorize(userCode);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-poster-ink/75 p-4">
      <div className="bg-poster-cream border-2 border-poster-ink shadow-hard w-full max-w-md p-6 relative rounded-none">
        {/* Close Button */}
        <button
          type="button"
          onClick={onClose}
          className="poster-btn absolute top-4 right-4 p-1.5 bg-white hover:bg-poster-red hover:text-white text-poster-ink"
          title="CLOSE"
        >
          <IconClose size={16} />
        </button>

        {/* Header */}
        <div className="mb-5 pb-3 border-b-2 border-poster-ink">
          <div className="font-mono text-[10px] font-bold text-poster-deepred uppercase tracking-widest">
            RFC 8628 / DEVICE FLOW
          </div>
          <h3 className="font-bungee text-2xl text-poster-ink tracking-wide mt-0.5">
            PAIR DEVICE
          </h3>
          <p className="font-mono text-xs text-poster-ink uppercase mt-1">
            LINK TERMINAL SESSION TO @{userHandle.toUpperCase()}
          </p>
        </div>

        {/* Pending Terminal Detection Banner */}
        {pendingDevices.length > 0 && (
          <div className="p-3 bg-poster-yellow border-2 border-poster-ink shadow-hard-sm mb-5 flex items-center justify-between gap-3">
            <div>
              <div className="font-mono text-[11px] font-bold text-poster-ink uppercase">
                PENDING REQUEST
              </div>
              <div className="font-mono text-sm font-bold text-poster-ink mt-0.5">
                CODE: {pendingDevices[0].user_code}
              </div>
            </div>
            <button
              type="button"
              disabled={status === 'authorizing'}
              onClick={() => handleAutoAuthorize(pendingDevices[0].user_code)}
              className="poster-btn px-3 py-1.5 bg-poster-mustard hover:bg-white text-poster-ink font-bungee text-xs uppercase"
            >
              1-CLICK PAIR
            </button>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label
              htmlFor="pairing-code"
              className="block font-mono text-xs font-bold text-poster-ink uppercase tracking-wider mb-2"
            >
              ENTER PAIRING CODE
            </label>
            <input
              id="pairing-code"
              type="text"
              placeholder="CNET-XXXX-XXXX"
              value={userCode}
              onChange={e => setUserCode(e.target.value)}
              className="w-full px-4 py-3 bg-white border-2 border-poster-ink text-center font-mono text-base sm:text-lg font-bold tracking-widest uppercase text-poster-ink shadow-hard-sm focus:outline-none focus:bg-poster-cream rounded-none placeholder:text-poster-ink/30"
            />
          </div>

          {status === 'error' && (
            <div className="p-3 bg-poster-red text-white border-2 border-poster-ink font-mono text-xs uppercase font-bold">
              ERROR: {errorMessage}
            </div>
          )}

          {status === 'success' && (
            <div className="p-3 bg-poster-green text-white border-2 border-poster-ink font-mono text-xs uppercase font-bold">
              DEVICE LINKED SUCCESSFULLY
            </div>
          )}

          <div className="flex gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="poster-btn flex-1 px-4 py-2.5 bg-white hover:bg-poster-cream text-poster-ink font-mono text-xs uppercase font-bold"
            >
              CANCEL
            </button>
            <button
              type="submit"
              disabled={status === 'authorizing'}
              className="poster-btn flex-1 px-4 py-2.5 bg-poster-mustard hover:bg-poster-yellow disabled:opacity-50 text-poster-ink font-bungee text-sm uppercase"
            >
              {status === 'authorizing' ? 'VERIFYING…' : 'AUTHORIZE'}
            </button>
          </div>

          {pendingDevices.length === 0 && (
            <div className="pt-2 text-center border-t border-poster-ink">
              <button
                type="button"
                disabled={status === 'authorizing'}
                onClick={() => handleAutoAuthorize()}
                className="font-mono text-xs font-bold text-poster-deepred hover:underline uppercase inline-block py-1"
              >
                AUTO-AUTHORIZE LATEST LOCAL TERMINAL
              </button>
            </div>
          )}
        </form>
      </div>
    </div>
  );
};
