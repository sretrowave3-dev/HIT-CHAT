import React, { useEffect, useState } from 'react';
import { Flame } from 'lucide-react';

interface CountdownTimerProps {
  timestampMS: number;
  ttlMS: number;
  onExpire: () => void;
}

export const CountdownTimer: React.FC<CountdownTimerProps> = ({
  timestampMS,
  ttlMS,
  onExpire,
}) => {
  const [remaining, setRemaining] = useState<number>(() => {
    const elapsed = Date.now() - timestampMS;
    return Math.max(0, ttlMS - elapsed);
  });

  useEffect(() => {
    const interval = setInterval(() => {
      const elapsed = Date.now() - timestampMS;
      const left = Math.max(0, ttlMS - elapsed);
      setRemaining(left);
      if (left <= 0) {
        clearInterval(interval);
        onExpire();
      }
    }, 100);

    return () => clearInterval(interval);
  }, [timestampMS, ttlMS, onExpire]);

  const percentage = Math.max(0, Math.min(100, (remaining / ttlMS) * 100));
  const seconds = (remaining / 1000).toFixed(1);

  const isLow = remaining < 3000;

  return (
    <div className="flex items-center gap-1.5 text-xs font-mono font-medium">
      <Flame className={`w-3.5 h-3.5 ${isLow ? 'text-red-500 animate-bounce' : 'text-amber-400'}`} />
      <span className={isLow ? 'text-red-400 font-bold' : 'text-gray-400'}>
        {seconds}s
      </span>
      <div className="w-12 h-1.5 bg-gray-800 rounded-full overflow-hidden border border-gray-700">
        <div
          className={`h-full transition-all duration-100 ${
            isLow ? 'bg-red-500' : 'bg-emerald-400'
          }`}
          style={{ width: `${percentage}%` }}
        />
      </div>
    </div>
  );
};
