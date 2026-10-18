import React, { useState } from 'react';
import { Terminal, Check, AlertCircle, X } from 'lucide-react';

interface DevicePairingProps {
  userHandle: string;
  isOpen: boolean;
  onClose: () => void;
}

export const DevicePairing: React.FC<DevicePairingProps> = ({
  userHandle,
  isOpen,
  onClose,
}) => {
  const [userCode, setUserCode] = useState('');
  const [status, setStatus] = useState<'idle' | 'authorizing' | 'success' | 'error'>('idle');
  const [errorMessage, setErrorMessage] = useState('');

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!userCode.trim()) return;

    setStatus('authorizing');
    setErrorMessage('');

    try {
      const res = await fetch('/api/v1/auth/device/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_code: userCode.trim().toUpperCase(),
          user_handle: userHandle,
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to authorize device');
      }

      setStatus('success');
      setTimeout(() => {
        onClose();
        setStatus('idle');
        setUserCode('');
      }, 1500);
    } catch (err: any) {
      setStatus('error');
      setErrorMessage(err.message || 'Verification error');
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
      <div className="bg-cyber-800 border border-cyber-700 w-full max-w-md rounded-2xl p-6 shadow-2xl relative">
        <button
          onClick={onClose}
          className="absolute top-4 right-4 text-gray-400 hover:text-gray-200"
        >
          <X className="w-5 h-5" />
        </button>

        <div className="flex items-center gap-3 mb-4">
          <div className="p-2.5 bg-emerald-500/10 text-emerald-400 rounded-xl border border-emerald-500/30">
            <Terminal className="w-6 h-6" />
          </div>
          <div>
            <h3 className="text-lg font-bold text-gray-100">Authorize CLI Terminal</h3>
            <p className="text-xs text-gray-400">Link headless terminal session to @{userHandle}</p>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-gray-300 uppercase tracking-wider mb-1.5">
              Terminal Pairing Code
            </label>
            <input
              type="text"
              placeholder="e.g. CNET-8429-K7P2"
              value={userCode}
              onChange={e => setUserCode(e.target.value)}
              className="w-full px-4 py-2.5 bg-cyber-900 border border-gray-700 rounded-xl text-center font-mono text-base tracking-widest uppercase text-emerald-400 focus:outline-none focus:border-emerald-500"
            />
          </div>

          {status === 'error' && (
            <div className="flex items-center gap-2 p-3 bg-red-500/10 border border-red-500/30 rounded-xl text-xs text-red-400">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{errorMessage}</span>
            </div>
          )}

          {status === 'success' && (
            <div className="flex items-center gap-2 p-3 bg-emerald-500/10 border border-emerald-500/30 rounded-xl text-xs text-emerald-300 font-semibold">
              <Check className="w-4 h-4 shrink-0" />
              <span>Terminal device linked successfully!</span>
            </div>
          )}

          <div className="flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2.5 bg-cyber-700 hover:bg-cyber-600 text-gray-300 text-sm font-semibold rounded-xl transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={status === 'authorizing'}
              className="flex-1 px-4 py-2.5 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white text-sm font-semibold rounded-xl shadow-lg shadow-emerald-900/30 transition"
            >
              {status === 'authorizing' ? 'Verifying...' : 'Authorize Device'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
