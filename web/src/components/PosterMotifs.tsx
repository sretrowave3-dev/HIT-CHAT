import React from 'react';
import { motion } from 'framer-motion';

// Motif 1: Hand-built Flat Lotus (red #D7191F, yellow #F7D63C, green #2E8B3D, ink #2A0A08)
export const LotusMotif: React.FC<{ className?: string; size?: number }> = ({ className = '', size = 24 }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={`inline-block shrink-0 ${className}`}
  >
    {/* Center petal */}
    <path
      d="M12 2C10 6 9 9 9 13C9 17 12 19 12 19C12 19 15 17 15 13C15 9 14 6 12 2Z"
      fill="#D7191F"
      stroke="#2A0A08"
      strokeWidth="1.5"
      strokeLinejoin="round"
    />
    {/* Left petal */}
    <path
      d="M10 8C7 9.5 4 12 5 15C5.8 17.5 9 18 10 18C9 16 9 12 10 8Z"
      fill="#F7D63C"
      stroke="#2A0A08"
      strokeWidth="1.5"
      strokeLinejoin="round"
    />
    {/* Right petal */}
    <path
      d="M14 8C17 9.5 20 12 19 15C18.2 17.5 15 18 14 18C15 16 15 12 14 8Z"
      fill="#F7D63C"
      stroke="#2A0A08"
      strokeWidth="1.5"
      strokeLinejoin="round"
    />
    {/* Base calyx */}
    <path
      d="M6 19C9 22 15 22 18 19C15 20 9 20 6 19Z"
      fill="#2E8B3D"
      stroke="#2A0A08"
      strokeWidth="1.5"
    />
  </svg>
);

// Motif 2: Hand-built Flat Chilli (red #D7191F, green #2E8B3D, ink #2A0A08)
export const ChilliMotif: React.FC<{ className?: string; size?: number }> = ({ className = '', size = 24 }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={`inline-block shrink-0 ${className}`}
  >
    {/* Green Stem */}
    <path
      d="M15 3C15 4 14 5 12 5.5"
      stroke="#2E8B3D"
      strokeWidth="2"
      strokeLinecap="round"
    />
    <path
      d="M10 5C11 4.5 13 4.5 14 5L13.5 7H10.5L10 5Z"
      fill="#2E8B3D"
      stroke="#2A0A08"
      strokeWidth="1.2"
    />
    {/* Red Body */}
    <path
      d="M11 6.5C14 7 17 9 17 12C17 15 14 19 9 21C8 21.5 7.5 21 8 20C10 16 12 13 11 6.5Z"
      fill="#D7191F"
      stroke="#2A0A08"
      strokeWidth="1.5"
      strokeLinejoin="round"
    />
  </svg>
);

// Motif 3: Hand-built Flat Chevron (Truck-art double zigzag accent)
export const ChevronMotif: React.FC<{ className?: string; size?: number }> = ({
  className = '',
  size = 18,
}) => (
  <svg
    width={size * 1.5}
    height={size}
    viewBox="0 0 28 18"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={`inline-block shrink-0 ${className}`}
  >
    <path
      d="M2 2L10 9L2 16"
      stroke="#2A0A08"
      strokeWidth="2.5"
      strokeLinecap="square"
      strokeLinejoin="miter"
    />
    <path
      d="M10 2L18 9L10 16"
      stroke="#F2A33A"
      strokeWidth="2.5"
      strokeLinecap="square"
      strokeLinejoin="miter"
    />
    <path
      d="M18 2L26 9L18 16"
      stroke="#D7191F"
      strokeWidth="2.5"
      strokeLinecap="square"
      strokeLinejoin="miter"
    />
  </svg>
);

// Motif 4: Four-Point Diamond Star Sparkle
export const SparkleMotif: React.FC<{ className?: string; size?: number; color?: string }> = ({
  className = '',
  size = 18,
  color = '#F7D63C',
}) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={`inline-block shrink-0 ${className}`}
  >
    <path
      d="M12 1L14.5 9.5L23 12L14.5 14.5L12 23L9.5 14.5L1 12L9.5 9.5L12 1Z"
      fill={color}
      stroke="#2A0A08"
      strokeWidth="1.5"
      strokeLinejoin="round"
    />
  </svg>
);

// Motif 5: Evil-Eye Charm / Talisman (Nazar protection motif with cobalt, cream, yellow, ink)
export const CharmMotif: React.FC<{ className?: string; size?: number }> = ({ className = '', size = 24 }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={`inline-block shrink-0 ${className}`}
  >
    {/* Outer eye contour / lashes */}
    <path
      d="M2 12C5 7 19 7 22 12C19 17 5 17 2 12Z"
      fill="#2457E0"
      stroke="#2A0A08"
      strokeWidth="1.5"
    />
    {/* White / cream inner oval */}
    <ellipse cx="12" cy="12" rx="6.5" ry="4.5" fill="#F6EACB" stroke="#2A0A08" strokeWidth="1" />
    {/* Mustard iris */}
    <circle cx="12" cy="12" r="3" fill="#F7D63C" stroke="#2A0A08" strokeWidth="1" />
    {/* Dark ink pupil */}
    <circle cx="12" cy="12" r="1.5" fill="#2A0A08" />
    {/* Top crown ray */}
    <path d="M12 4V2M8 5L7 3M16 5L17 3" stroke="#2A0A08" strokeWidth="1.2" strokeLinecap="round" />
  </svg>
);

// Motif 6: Paisley / Mango (Ambi / Kairi Indian folk print motif)
export const PaisleyMotif: React.FC<{ className?: string; size?: number }> = ({ className = '', size = 24 }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={`inline-block shrink-0 ${className}`}
  >
    <path
      d="M12 2C8 2 5 6 5 11C5 16 8.5 20 13 20C17.5 20 20 16.5 20 12C20 6.5 14.5 8 13.5 4C13 2.8 12.6 2 12 2Z"
      fill="#F2A33A"
      stroke="#2A0A08"
      strokeWidth="1.5"
      strokeLinejoin="round"
    />
    <circle cx="12" cy="13" r="3" fill="#D7191F" stroke="#2A0A08" strokeWidth="1" />
    <circle cx="12" cy="13" r="1" fill="#F7D63C" />
  </svg>
);

// Plain functional icons drawn in ink with 2px stroke (no external icon libraries)
export const IconSend: React.FC<{ className?: string; size?: number }> = ({ className = '', size = 18 }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke="#2A0A08"
    strokeWidth="2"
    strokeLinecap="square"
    strokeLinejoin="miter"
    className={className}
  >
    <path d="M22 2L11 13" />
    <path d="M22 2L15 22L11 13L2 9L22 2Z" />
  </svg>
);

export const IconClose: React.FC<{ className?: string; size?: number }> = ({ className = '', size = 18 }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke="#2A0A08"
    strokeWidth="2"
    strokeLinecap="square"
    strokeLinejoin="miter"
    className={className}
  >
    <line x1="18" y1="6" x2="6" y2="18" />
    <line x1="6" y1="6" x2="18" y2="18" />
  </svg>
);

export const IconTrash: React.FC<{ className?: string; size?: number }> = ({ className = '', size = 16 }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke="#2A0A08"
    strokeWidth="2"
    strokeLinecap="square"
    strokeLinejoin="miter"
    className={className}
  >
    <polyline points="3 6 5 6 21 6" />
    <path d="M19 6V20C19 20.5 18.5 21 18 21H6C5.5 21 5 20.5 5 20V6" />
    <path d="M10 11V17" />
    <path d="M14 11V17" />
    <path d="M9 6V4C9 3.5 9.5 3 10 3H14C14.5 3 15 3.5 15 4V6" />
  </svg>
);

export const IconPlus: React.FC<{ className?: string; size?: number }> = ({ className = '', size = 16 }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke="#2A0A08"
    strokeWidth="2"
    strokeLinecap="square"
    className={className}
  >
    <line x1="12" y1="5" x2="12" y2="19" />
    <line x1="5" y1="12" x2="19" y2="12" />
  </svg>
);

export const IconLock: React.FC<{ className?: string; size?: number }> = ({ className = '', size = 14 }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke="#2A0A08"
    strokeWidth="2"
    strokeLinecap="square"
    strokeLinejoin="miter"
    className={className}
  >
    <rect x="3" y="11" width="18" height="11" rx="0" ry="0" />
    <path d="M7 11V7C7 4.2 9.2 2 12 2C14.8 2 17 4.2 17 7V11" />
  </svg>
);

export const IconInfo: React.FC<{ className?: string; size?: number }> = ({ className = '', size = 16 }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke="#2A0A08"
    strokeWidth="2"
    strokeLinecap="square"
    strokeLinejoin="miter"
    className={className}
  >
    <circle cx="12" cy="12" r="10" />
    <path d="M12 16V12" />
    <path d="M12 8H12.01" />
  </svg>
);

// Interactive Draggable Sticker Component with Spring Physics
interface DraggableStickerProps {
  label?: string;
  motif?: 'lotus' | 'chilli' | 'sparkle' | 'charm' | 'paisley';
  size?: number;
  initialRotate?: number;
  className?: string;
}

export const DraggableSticker: React.FC<DraggableStickerProps> = ({
  label,
  motif = 'lotus',
  size = 28,
  initialRotate = 0,
  className = '',
}) => {
  const renderMotif = () => {
    switch (motif) {
      case 'lotus':
        return <LotusMotif size={size} />;
      case 'chilli':
        return <ChilliMotif size={size} />;
      case 'sparkle':
        return <SparkleMotif size={size} />;
      case 'charm':
        return <CharmMotif size={size} />;
      case 'paisley':
        return <PaisleyMotif size={size} />;
      default:
        return <LotusMotif size={size} />;
    }
  };

  return (
    <motion.div
      drag
      dragElastic={0.2}
      whileHover={{ scale: 1.12, rotate: initialRotate + 4 }}
      whileTap={{ scale: 0.95 }}
      transition={{ type: 'spring', stiffness: 400, damping: 20 }}
      style={{ rotate: initialRotate }}
      className={`inline-flex items-center gap-1.5 px-2.5 py-1 bg-white border-2 border-poster-ink shadow-hard rounded-[14px_4px_14px_4px] cursor-grab active:cursor-grabbing select-none shrink-0 z-20 ${className}`}
      title="DRAG ME!"
    >
      {renderMotif()}
      {label && (
        <span className="font-bungee text-[11px] text-poster-ink uppercase tracking-wider">
          {label}
        </span>
      )}
    </motion.div>
  );
};
