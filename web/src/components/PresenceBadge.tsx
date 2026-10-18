import React from 'react';
import { Shield, ShieldAlert, Wifi, MapPin } from 'lucide-react';

interface PresenceBadgeProps {
  locationEnabled: boolean;
  zoneDisplayName: string;
  buildingName?: string;
  onTogglePrivacy: () => void;
}

export const PresenceBadge: React.FC<PresenceBadgeProps> = ({
  locationEnabled,
  zoneDisplayName,
  buildingName,
  onTogglePrivacy,
}) => {
  return (
    <div className="flex items-center justify-between p-3 bg-cyber-800 border border-cyber-700 rounded-xl shadow-lg">
      <div className="flex items-center gap-3">
        <div
          className={`p-2 rounded-lg ${
            locationEnabled
              ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
              : 'bg-amber-500/10 text-amber-400 border border-amber-500/30'
          }`}
        >
          {locationEnabled ? <Wifi className="w-5 h-5 animate-pulse" /> : <Shield className="w-5 h-5" />}
        </div>
        <div>
          <div className="flex items-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-wider text-gray-400">
              Campus Presence
            </span>
            <span
              className={`px-1.5 py-0.5 text-[10px] font-bold rounded ${
                locationEnabled
                  ? 'bg-emerald-500/20 text-emerald-300'
                  : 'bg-amber-500/20 text-amber-300'
              }`}
            >
              {locationEnabled ? 'Broadcasting AP Hash' : 'Privacy Zero-Bypass'}
            </span>
          </div>
          <div className="flex items-center gap-1.5 text-sm font-medium text-gray-200 mt-0.5">
            {locationEnabled ? (
              <>
                <MapPin className="w-3.5 h-3.5 text-emerald-400" />
                <span>{zoneDisplayName}</span>
                {buildingName && <span className="text-gray-400 text-xs">({buildingName})</span>}
              </>
            ) : (
              <>
                <ShieldAlert className="w-3.5 h-3.5 text-amber-400" />
                <span className="text-amber-300">Protected Mode (Null Vectors Dispatched)</span>
              </>
            )}
          </div>
        </div>
      </div>

      <button
        onClick={onTogglePrivacy}
        className={`px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors border ${
          locationEnabled
            ? 'bg-cyber-700 hover:bg-cyber-600 text-gray-300 border-gray-600'
            : 'bg-emerald-600 hover:bg-emerald-500 text-white border-emerald-500 shadow-md shadow-emerald-900/30'
        }`}
      >
        {locationEnabled ? 'Conceal Location' : 'Share Campus Zone'}
      </button>
    </div>
  );
};
