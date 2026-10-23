import React from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { IconLock, SparkleMotif } from './PosterMotifs.tsx';

interface DiagnosticsModalProps {
  isOpen: boolean;
  onClose: () => void;
  isConnected: boolean;
  currentUser: string;
  currentContact: string;
  lastAckedId: string;
  outboxLength: number;
}

export const DiagnosticsModal: React.FC<DiagnosticsModalProps> = ({
  isOpen,
  onClose,
  isConnected,
  currentUser,
  currentContact,
  lastAckedId,
  outboxLength,
}) => {
  return (
    <AnimatePresence>
      {isOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-poster-ink/60 backdrop-blur-[2px]">
          <motion.div
            initial={{ scale: 0.95, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ scale: 0.95, opacity: 0 }}
            className="w-full max-w-md bg-poster-cream paper-surface border-2 border-poster-ink shadow-hard p-5 sm:p-6 rounded-[24px_6px_24px_6px] relative"
          >
            <div className="flex items-center justify-between border-b-2 border-poster-ink pb-3 mb-4">
              <div className="flex items-center gap-2">
                <SparkleMotif size={16} color="#C8141A" />
                <h3 className="font-bungee text-lg text-poster-ink uppercase tracking-wide">
                  SYSTEM DIAGNOSTICS
                </h3>
              </div>
              <button
                type="button"
                onClick={onClose}
                className="font-mono text-xs font-bold px-2 py-1 bg-white border border-poster-ink shadow-hard-sm uppercase hover:bg-poster-red hover:text-white"
              >
                [X]
              </button>
            </div>

            <div className="space-y-3 font-mono text-xs text-poster-ink">
              <div className="flex justify-between items-center py-1.5 border-b border-poster-ink/20">
                <span className="font-bold">WEBSOCKET STATUS:</span>
                <span className={`px-2 py-0.5 border border-poster-ink font-bold ${
                  isConnected ? 'bg-poster-green text-poster-cream' : 'bg-poster-red text-white'
                }`}>
                  {isConnected ? 'ONLINE / CONNECTED' : 'OFFLINE / RECONNECTING'}
                </span>
              </div>

              <div className="flex justify-between items-center py-1.5 border-b border-poster-ink/20">
                <span className="font-bold">LOCAL IDENTITY:</span>
                <span className="bg-white px-2 py-0.5 border border-poster-ink font-bold">
                  @{currentUser}
                </span>
              </div>

              <div className="flex justify-between items-center py-1.5 border-b border-poster-ink/20">
                <span className="font-bold">CURRENT CONTACT:</span>
                <span className="bg-white px-2 py-0.5 border border-poster-ink font-bold">
                  {currentContact}
                </span>
              </div>

              <div className="flex justify-between items-center py-1.5 border-b border-poster-ink/20">
                <span className="font-bold">OFFLINE OUTBOX:</span>
                <span className="bg-white px-2 py-0.5 border border-poster-ink font-bold">
                  {outboxLength} ENVELOPES QUEUED
                </span>
              </div>

              <div className="flex flex-col py-1.5 border-b border-poster-ink/20 gap-1">
                <span className="font-bold">LAST ACKNOWLEDGED ID:</span>
                <span className="bg-white px-2 py-1 border border-poster-ink text-[11px] truncate select-all">
                  {lastAckedId || 'none'}
                </span>
              </div>

              <div className="flex justify-between items-center py-1.5 border-b border-poster-ink/20">
                <span className="font-bold">CRYPTO ENGINE:</span>
                <span className="text-[11px] font-bold text-poster-deepred">
                  AES-256-GCM / RFC-5869 HKDF
                </span>
              </div>

              <div className="p-3 bg-poster-green/15 border-2 border-poster-green rounded-[12px_4px_12px_4px] mt-4">
                <div className="flex items-center gap-1.5 font-bold text-poster-ink mb-1">
                  <IconLock size={13} />
                  <span>ZERO-KNOWLEDGE VERIFIED</span>
                </div>
                <p className="text-[11px] leading-relaxed text-poster-ink/80">
                  Private keys and plaintexts never touch backend logs or wire transport. Plaintext is decrypted purely client-side in RAM.
                </p>
              </div>
            </div>

            <div className="mt-5 flex justify-end">
              <button
                type="button"
                onClick={onClose}
                className="poster-btn px-4 py-2 bg-poster-mustard hover:bg-poster-yellow text-poster-ink font-bungee text-xs uppercase rounded-[12px_4px_12px_4px]"
              >
                CLOSE
              </button>
            </div>
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  );
};
