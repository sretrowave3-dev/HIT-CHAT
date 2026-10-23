import React from 'react';
import { IconClose } from './PosterMotifs.tsx';

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
  if (!isOpen) return null;

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
        <div className="mb-4 pb-3 border-b-2 border-poster-ink">
          <div className="font-mono text-[10px] font-bold text-poster-deepred uppercase tracking-widest">
            PRIVACY SPECIFICATION / ZERO-BYPASS
          </div>
          <h3 className="font-bungee text-2xl text-poster-ink tracking-wide mt-0.5">
            LOCATION PRIVACY
          </h3>
        </div>

        {/* Content */}
        <div className="space-y-4">
          <div className="p-3 bg-white border-2 border-poster-ink">
            <div className="font-mono text-xs font-bold text-poster-ink uppercase">
              CURRENT ZONE STATUS:
            </div>
            <div className="font-mono text-sm font-bold text-poster-deepred mt-1 uppercase">
              {locationEnabled
                ? `${zoneDisplayName}${buildingName ? ` (${buildingName})` : ''}`
                : 'PRIVATE / NULL VECTORS DISPATCHED'}
            </div>
          </div>

          <p className="font-body text-[15px] text-poster-ink leading-relaxed">
            When location is concealed, the client transmits zeroed telemetry vectors.
            When broadcasting, Wi-Fi BSSID access point identifiers are HMAC-hashed with
            campus salt prior to dispatch.
          </p>

          <div className="pt-2">
            <button
              type="button"
              onClick={onTogglePrivacy}
              className="poster-btn w-full px-4 py-2.5 bg-poster-mustard hover:bg-poster-yellow text-poster-ink font-bungee text-xs uppercase"
            >
              {locationEnabled ? 'CONCEAL LOCATION (NULL VECTORS)' : 'BROADCAST CAMPUS ZONE'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

// Also export as PresenceBadge for backward-compatibility if imported
export const PresenceBadge = PrivacyInfoModal;
