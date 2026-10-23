import React, { useEffect, useState, useRef } from 'react';
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
  // Capture local arrival timestamp so client clock skew against server never causes instant auto-burn
  const arrivalTimeRef = useRef<number>(Date.now());
  const effectiveTTL = Math.max(3000, ttlMS || 10000); // Minimum 3s reading window

  const computeRemaining = () => {
    const now = Date.now();
    const clientElapsed = now - arrivalTimeRef.current;
    const serverElapsed = now - (timestampMS || arrivalTimeRef.current);

    // If server clock has skew (future or past), prioritize local elapsed reading time
    let elapsed = clientElapsed;
    if (serverElapsed > 0 && Math.abs(serverElapsed - clientElapsed) < 5000) {
      elapsed = Math.max(clientElapsed, serverElapsed);
    }
    return Math.max(0, effectiveTTL - elapsed);
  };

  const [remaining, setRemaining] = useState<number>(computeRemaining);
  const onExpireRef = useRef(onExpire);
  onExpireRef.current = onExpire;

  useEffect(() => {
    const interval = setInterval(() => {
      const left = computeRemaining();
      setRemaining(left);
      if (left <= 0) {
        clearInterval(interval);
        onExpireRef.current();
      }
    }, 100);

    return () => clearInterval(interval);
  }, [timestampMS, effectiveTTL]);

  const percentage = Math.max(0, Math.min(100, (remaining / effectiveTTL) * 100));
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
