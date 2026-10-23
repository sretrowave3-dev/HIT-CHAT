import React, { useEffect, useState, useRef } from 'react';

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

  const fraction = Math.max(0, Math.min(1, remaining / effectiveTTL));
  const seconds = (remaining / 1000).toFixed(1);
  const totalBlocks = 8;
  const activeBlocks = Math.ceil(fraction * totalBlocks);

  // Color drops from green -> yellow/mustard -> red as time elapses
  let blockColor = 'bg-poster-green';
  if (fraction < 0.3) {
    blockColor = 'bg-poster-red';
  } else if (fraction < 0.6) {
    blockColor = 'bg-poster-mustard';
  }

  return (
    <div
      className="inline-flex items-center gap-2 font-mono text-xs uppercase"
      title={`BURN IN ${seconds}s`}
    >
      {/* Segmented bar of small square blocks */}
      <div className="flex items-center gap-0.5" aria-hidden="true">
        {Array.from({ length: totalBlocks }).map((_, i) => {
          const isFilled = i < activeBlocks;
          return (
            <div
              key={i}
              className={`w-2 h-2.5 border border-poster-ink ${
                isFilled ? blockColor : 'bg-transparent opacity-25'
              }`}
            />
          );
        })}
      </div>

      <span className="font-bold text-poster-ink tracking-tight">
        {seconds}s
      </span>
    </div>
  );
};
