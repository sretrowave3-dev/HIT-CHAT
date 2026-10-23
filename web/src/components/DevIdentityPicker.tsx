import React, { useState } from 'react';
import { motion } from 'framer-motion';
import { canonicalHandle } from '../crypto/ratchet.ts';

interface DevIdentityPickerProps {
  currentHandle: string;
  onSwitchIdentity: (handle: string, token: string) => void;
}

const DEV_IDENTITIES = [
  { handle: 'alice', label: '@ALICE (WEB)', role: 'Lead Student' },
  { handle: 'bob', label: '@BOB (LAB)', role: 'Researcher' },
  { handle: 'carol', label: '@CAROL', role: 'TA / Faculty' },
  { handle: 'student_guest', label: '@STUDENT_GUEST (CLI)', role: 'Terminal Guest' },
];

export const DevIdentityPicker: React.FC<DevIdentityPickerProps> = ({
  currentHandle,
  onSwitchIdentity,
}) => {
  const [loadingHandle, setLoadingHandle] = useState<string | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const handleSelect = async (handle: string) => {
    if (loadingHandle) return;
    setLoadingHandle(handle);
    setErrorMsg(null);

    try {
      const res = await fetch('/api/v1/dev/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ handle }),
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `Dev login returned ${res.status}`);
      }

      const data = await res.json();
      if (data.access_token) {
        onSwitchIdentity(data.user_handle || handle, data.access_token);
      }
    } catch (err: any) {
      console.warn('[DevAuth Error]', err);
      setErrorMsg(err?.message || 'Dev login failed');
    } finally {
      setLoadingHandle(null);
    }
  };

  const isDevAuthEnabled = import.meta.env.VITE_DEV_AUTH === 'true' || import.meta.env.DEV;
  if (!isDevAuthEnabled) {
    return null;
  }

  return (
    <div className="bg-poster-yellow border-2 border-poster-ink shadow-hard p-2.5 sm:p-3 rounded-[16px_4px_16px_4px] flex flex-col sm:flex-row items-center justify-between gap-3 z-20">
      <div className="flex items-center gap-2">
        <span className="font-mono text-[10px] bg-poster-deepred text-poster-cream font-bold px-1.5 py-0.5 rounded-none uppercase tracking-widest">
          DEV-AUTH
        </span>
        <span className="font-mono text-xs font-bold text-poster-ink uppercase">
          SWITCH TEST IDENTITY:
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {DEV_IDENTITIES.map(id => {
          const isActive = canonicalHandle(currentHandle) === canonicalHandle(id.handle);
          const isLoading = loadingHandle === id.handle;

          return (
            <motion.button
              key={id.handle}
              whileTap={{ scale: 0.95 }}
              type="button"
              onClick={() => handleSelect(id.handle)}
              disabled={isLoading || isActive}
              className={`font-mono text-xs font-bold px-2.5 py-1 uppercase border border-poster-ink shadow-hard-sm rounded-[6px_2px_6px_2px] transition-all cursor-pointer ${
                isActive
                  ? 'bg-poster-green text-poster-cream cursor-default shadow-none'
                  : 'bg-white hover:bg-poster-mustard text-poster-ink'
              }`}
            >
              {isLoading ? 'SWITCHING…' : id.label}
            </motion.button>
          );
        })}
      </div>

      {errorMsg && (
        <span className="font-mono text-[11px] text-poster-red font-bold">
          {errorMsg}
        </span>
      )}
    </div>
  );
};
