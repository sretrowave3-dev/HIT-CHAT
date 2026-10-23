import React from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { IconClose, SparkleMotif, LotusMotif } from './PosterMotifs.tsx';

interface PrivacyInfoModalProps {
  isOpen: boolean;
  onClose: () => void;
  locationEnabled: boolean;
  zoneDisplayName: string;
  buildingName?: string;
  onTogglePrivacy: () => void;
}

export const PrivacyInfoModal: React.FC<PrivacyInfoModalProps> = ({
  isOpen,
  onClose,
  locationEnabled,
  zoneDisplayName,
  buildingName,
  onTogglePrivacy,
}) => {
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
            {/* Halftone corner accent */}
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
                <span>PRIVACY SPECIFICATION / ZERO-BYPASS</span>
              </div>
              <h3 className="font-bungee text-2xl text-poster-ink tracking-wide mt-1">
                LOCATION PRIVACY
              </h3>
            </div>

            {/* Content */}
            <div className="space-y-4 relative z-10">
              <div className="p-3 bg-white border-2 border-poster-ink rounded-[16px_4px_16px_4px] shadow-hard-sm">
                <div className="font-mono text-xs font-bold text-poster-ink uppercase">
                  CURRENT BROADCAST STATE:
                </div>
                <div className="font-mono text-sm font-bold text-poster-deepred mt-1 uppercase flex items-center gap-2">
                  <span
                    className={`w-2.5 h-2.5 rounded-full border border-poster-ink ${
                      locationEnabled ? 'bg-poster-green' : 'bg-poster-red'
                    }`}
                  />
                  <span>
                    {locationEnabled
                      ? `${zoneDisplayName}${buildingName ? ` (${buildingName})` : ''}`
                      : 'PRIVATE MODE / NULL VECTORS'}
                  </span>
                </div>
              </div>

              <p className="font-body text-[15px] text-poster-ink leading-relaxed">
                When location is concealed, the client transmits zeroed telemetry vectors. When
                broadcasting, Wi-Fi BSSID access point identifiers are HMAC-hashed with campus salt
                prior to dispatch. No raw GPS coordinates or device identifiers ever reach the wire.
              </p>

              <div className="pt-2">
                <motion.button
                  whileTap={{ scale: 0.95 }}
                  type="button"
                  onClick={onTogglePrivacy}
                  className="poster-btn w-full px-4 py-3 bg-poster-mustard hover:bg-poster-yellow text-poster-ink font-bungee text-xs uppercase rounded-[16px_4px_16px_4px] flex items-center justify-center gap-2"
                >
                  <LotusMotif size={18} />
                  <span>
                    {locationEnabled ? 'CONCEAL LOCATION (NULL VECTORS)' : 'BROADCAST CAMPUS ZONE'}
                  </span>
                </motion.button>
              </div>
            </div>
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  );
};

// Backward-compatibility export
export const PresenceBadge = PrivacyInfoModal;
