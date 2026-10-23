import React, { useState, useRef, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { CountdownTimer } from './CountdownTimer.tsx';
import { canonicalHandle } from '../crypto/ratchet.ts';
import {
  LotusMotif,
  ChevronMotif,
  SparkleMotif,
  CharmMotif,
  IconSend,
  IconTrash,
  IconLock,
} from './PosterMotifs.tsx';

export type MessageStatus = 'SENDING' | 'SENT' | 'DELIVERED' | 'READ' | 'FAILED' | 'BURNED';

export interface DisplayMessage {
  id: string;
  sender: string;
  recipient: string;
  plaintext: string;
  timestampMS: number;
  ttlMS: number;
  isBurning?: boolean;
  status?: MessageStatus;
  isDecryptError?: boolean;
  rawEnvelope?: any;
}

interface ChatFeedProps {
  currentContact: string;
  currentUser: string;
  messages: DisplayMessage[];
  isPeerTyping?: boolean;
  onSendMessage: (text: string, ttlMS: number) => void;
  onBurnMessage: (id: string) => void;
  onRetryDecrypt?: (msg: DisplayMessage) => void;
  onTyping?: (isTyping: boolean) => void;
}

// Bouncing letters for GUPSHUP headline
const GupshupLetters: React.FC<{ text: string }> = ({ text }) => {
  return (
    <span className="inline-flex cursor-default select-none">
      {text.split('').map((char, index) => (
        <motion.span
          key={index}
          whileHover={{
            y: -10,
            scale: 1.15,
            rotate: index % 2 === 0 ? 8 : -8,
            transition: { type: 'spring', stiffness: 500, damping: 12 },
          }}
          className="inline-block poster-split-title"
        >
          {char}
        </motion.span>
      ))}
    </span>
  );
};

// 3-Depth Parallax Arch Artwork
const ParallaxArchIllustration: React.FC = () => {
  const containerRef = useRef<HTMLDivElement>(null);
  const [offset, setOffset] = useState({ x: 0, y: 0 });

  const handleMouseMove = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!containerRef.current) return;
    const rect = containerRef.current.getBoundingClientRect();
    const centerX = rect.left + rect.width / 2;
    const centerY = rect.top + rect.height / 2;
    const deltaX = (e.clientX - centerX) / (rect.width / 2);
    const deltaY = (e.clientY - centerY) / (rect.height / 2);
    setOffset({
      x: Math.max(-1, Math.min(1, deltaX)),
      y: Math.max(-1, Math.min(1, deltaY)),
    });
  };

  const handleMouseLeave = () => {
    setOffset({ x: 0, y: 0 });
  };

  return (
    <div
      ref={containerRef}
      onMouseMove={handleMouseMove}
      onMouseLeave={handleMouseLeave}
      className="w-56 h-40 mx-auto mb-4 bg-poster-cobalt border-2 border-poster-ink shadow-hard-lg relative flex flex-col items-center justify-center overflow-hidden p-2 rounded-t-[36px] rounded-b-[6px] cursor-pointer"
      title="HOVER TO EXPLORE PARALLAX"
    >
      {/* Decorative inner arch border */}
      <div className="absolute inset-2 border-2 border-poster-yellow rounded-t-[28px] rounded-b-[2px] pointer-events-none" />

      {/* Layer 1: Background Rays & Halftone (Depth 1: offset * 3px) */}
      <motion.div
        className="absolute inset-0 flex items-center justify-center pointer-events-none"
        animate={{
          x: offset.x * 4,
          y: offset.y * 4,
        }}
        transition={{ type: 'spring', damping: 25, stiffness: 200 }}
      >
        <svg width="180" height="120" viewBox="0 0 180 120" fill="none" className="opacity-70">
          <line x1="90" y1="60" x2="90" y2="10" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
          <line x1="90" y1="60" x2="40" y2="20" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
          <line x1="90" y1="60" x2="140" y2="20" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
          <line x1="90" y1="60" x2="15" y2="60" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
          <line x1="90" y1="60" x2="165" y2="60" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
          <line x1="90" y1="60" x2="45" y2="100" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
          <line x1="90" y1="60" x2="135" y2="100" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
        </svg>
      </motion.div>

      {/* Layer 2: Midground Folk Wings & Geometric Arches (Depth 2: offset * 8px) */}
      <motion.div
        className="absolute inset-0 flex items-center justify-center pointer-events-none"
        animate={{
          x: offset.x * 8,
          y: offset.y * 8,
        }}
        transition={{ type: 'spring', damping: 20, stiffness: 250 }}
      >
        <svg width="140" height="90" viewBox="0 0 140 90" fill="none">
          {/* Symmetrical folk wings */}
          <path
            d="M70 48C56 32 32 38 18 54C34 56 52 54 70 48Z"
            fill="#F6EACB"
            stroke="#2A0A08"
            strokeWidth="1.8"
          />
          <path
            d="M70 48C84 32 108 38 122 54C106 56 88 54 70 48Z"
            fill="#F6EACB"
            stroke="#2A0A08"
            strokeWidth="1.8"
          />
          {/* Base decorative ripples */}
          <path
            d="M25 72C35 68 45 76 55 72C65 68 75 76 85 72C95 68 105 76 115 72"
            stroke="#F7D63C"
            strokeWidth="2.5"
            strokeLinecap="round"
          />
        </svg>
      </motion.div>

      {/* Layer 3: Foreground Lotus & Golden Sparkle Core (Depth 3: offset * 14px) */}
      <motion.div
        className="relative z-10 flex flex-col items-center justify-center"
        animate={{
          x: offset.x * 14,
          y: offset.y * 14,
        }}
        transition={{ type: 'spring', damping: 15, stiffness: 300 }}
      >
        <div className="relative">
          <LotusMotif size={44} />
          <motion.div
            className="absolute -top-1 -right-2"
            animate={{ rotate: [0, 90, 180, 270, 360], scale: [1, 1.2, 1] }}
            transition={{ repeat: Infinity, duration: 8, ease: 'linear' }}
          >
            <SparkleMotif size={16} color="#F7D63C" />
          </motion.div>
        </div>
        <div className="font-mono text-[9px] uppercase tracking-widest text-poster-cream font-bold mt-1 bg-poster-ink/40 px-2 py-0.5 border border-poster-yellow/40">
          PARALLAX / WIRE NO. 01
        </div>
      </motion.div>
    </div>
  );
};

// Tiny Riso Confetti Burst on Message Send
const SendConfettiBurst: React.FC = () => {
  const colors = ['#F7D63C', '#D7191F', '#2E8B3D', '#F2A33A'];
  return (
    <div className="absolute -top-3 right-6 pointer-events-none z-30">
      {Array.from({ length: 6 }).map((_, i) => {
        const angle = (i / 6) * 360;
        const dist = 28 + (i % 3) * 8;
        const rad = (angle * Math.PI) / 180;
        const x = Math.cos(rad) * dist;
        const y = Math.sin(rad) * dist;
        return (
          <motion.div
            key={i}
            initial={{ scale: 0, x: 0, y: 0, opacity: 1, rotate: 0 }}
            animate={{
              scale: [0, 1.2, 0],
              x,
              y,
              opacity: [1, 1, 0],
              rotate: [0, 90 + i * 45],
            }}
            transition={{ duration: 0.55, ease: 'easeOut' }}
            style={{ backgroundColor: colors[i % colors.length] }}
            className="absolute w-2 h-2 border border-poster-ink shadow-hard-sm"
          />
        );
      })}
    </div>
  );
};

// Petal Bouncing Typing / Ready Indicator
const PetalTypingIndicator: React.FC = () => {
  return (
    <div className="flex items-center gap-1.5 px-3 py-1 bg-white border border-poster-ink shadow-hard-sm rounded-[12px_4px_12px_4px]">
      <span className="font-mono text-[10px] uppercase font-bold text-poster-deepred">
        WIRE READY
      </span>
      <div className="flex items-center gap-1">
        {[0, 1, 2].map(i => (
          <motion.div
            key={i}
            animate={{
              y: [0, -5, 0],
              scale: [1, 1.15, 1],
            }}
            transition={{
              repeat: Infinity,
              duration: 0.9,
              delay: i * 0.18,
              ease: 'easeInOut',
            }}
            className="w-2 h-2 rounded-[50%_0_50%_50%] rotate-45 border border-poster-ink"
            style={{
              backgroundColor: i === 0 ? '#D7191F' : i === 1 ? '#F7D63C' : '#2E8B3D',
            }}
          />
        ))}
      </div>
    </div>
  );
};

export const ChatFeed: React.FC<ChatFeedProps> = ({
  currentContact,
  currentUser,
  messages,
  isPeerTyping,
  onSendMessage,
  onBurnMessage,
  onRetryDecrypt,
  onTyping,
}) => {
  const [inputText, setInputText] = useState('');
  const [ttlMS, setTtlMS] = useState(10000); // 10s default
  const [showSendBurst, setShowSendBurst] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const typingTimeoutRef = useRef<any>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages.length]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setInputText(e.target.value);
    if (onTyping) {
      onTyping(true);
      clearTimeout(typingTimeoutRef.current);
      typingTimeoutRef.current = setTimeout(() => {
        onTyping(false);
      }, 1500);
    }
  };

  const handleSend = (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputText.trim()) return;
    if (onTyping) onTyping(false);
    onSendMessage(inputText.trim(), ttlMS);
    setInputText('');
    setShowSendBurst(true);
    setTimeout(() => setShowSendBurst(false), 700);
  };

  const contactInitial = currentContact.replace('@', '').charAt(0).toUpperCase();

  return (
    <div className="flex flex-col h-full bg-poster-cream paper-surface border-2 border-poster-ink shadow-hard rounded-[28px_8px_28px_8px] overflow-hidden relative">
      {/* Halftone Dot Corner in Chat Area */}
      <div className="absolute top-0 right-0 w-32 h-32 poster-halftone-pattern rounded-tr-[24px]" />

      {/* Top Header */}
      <div className="flex items-center justify-between px-4 sm:px-6 py-3 border-b-2 border-poster-ink bg-poster-cream z-10">
        <div className="flex items-center gap-3">
          {/* Asymmetric Avatar with 2px ink border */}
          <div className="w-10 h-10 bg-poster-yellow border-2 border-poster-ink rounded-[12px_4px_12px_4px] flex items-center justify-center font-bungee text-poster-ink text-base shadow-hard-sm shrink-0">
            {contactInitial}
          </div>

          <div>
            <div className="flex items-center gap-2">
              <h2 className="font-bungee text-base sm:text-lg text-poster-ink uppercase tracking-wide">
                {currentContact}
              </h2>
              <span className="font-mono text-[11px] font-bold px-1.5 py-0.5 bg-poster-green text-poster-cream border border-poster-ink uppercase rounded-none">
                ACTIVE
              </span>
            </div>
            <div className="font-mono text-[11px] uppercase text-poster-deepred font-bold flex items-center gap-1.5">
              <IconLock size={11} />
              <span>PAIRWISE HKDF / ZERO-KNOWLEDGE</span>
            </div>
          </div>
        </div>

        {/* TTL Selector */}
        <div className="flex items-center gap-2">
          <label
            htmlFor="ttl-select"
            className="font-mono text-xs uppercase font-bold text-poster-ink hidden sm:inline"
          >
            BURN TIMER:
          </label>
          <select
            id="ttl-select"
            value={ttlMS}
            onChange={e => setTtlMS(Number(e.target.value))}
            className="font-mono text-xs font-bold uppercase bg-white border-2 border-poster-ink px-2.5 py-1.5 shadow-hard-sm focus:outline-none cursor-pointer rounded-[6px_2px_6px_2px]"
          >
            <option value={5000}>5 SECONDS</option>
            <option value={10000}>10 SECONDS</option>
            <option value={30000}>30 SECONDS</option>
            <option value={60000}>60 SECONDS</option>
          </select>
        </div>
      </div>

      {/* Message Feed Area */}
      <div className="flex-1 overflow-y-auto p-4 sm:p-6 space-y-4 bg-poster-cream/50 relative">
        {messages.length === 0 ? (
          /* Empty Chat State: Parallax Arch Artwork + Bouncing GUPSHUP headline + Devanagari subtitle */
          <div className="h-full flex flex-col items-center justify-center p-4">
            <motion.div
              initial={{ scale: 0.94, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              transition={{ type: 'spring', damping: 20 }}
              className="w-full max-w-md bg-poster-cream border-2 border-poster-ink shadow-hard p-6 sm:p-8 text-center relative rounded-[28px_8px_28px_8px]"
            >
              {/* Corner Motifs */}
              <div className="absolute top-3 left-3">
                <LotusMotif size={22} />
              </div>
              <div className="absolute top-3 right-3">
                <CharmMotif size={22} />
              </div>
              <div className="absolute bottom-3 left-3 rotate-180 scale-x-[-1]">
                <LotusMotif size={22} />
              </div>
              <div className="absolute bottom-3 right-3">
                <SparkleMotif size={22} />
              </div>

              {/* Symmetrical Arch Frame with 3-Depth Parallax */}
              <ParallaxArchIllustration />

              {/* Chunky 3D Bungee Headline with Bouncing Letters on Hover */}
              <h3 className="font-bungee text-3xl sm:text-4xl tracking-wider leading-none mt-2">
                <GupshupLetters text="GUPSHUP" />
              </h3>

              {/* Small Devanagari Subtitle */}
              <p className="font-yatra text-base text-poster-red mt-1 font-bold">
                à¤¶à¥‚à¤¨à¥à¤¯-à¤¸à¥à¤®à¥ƒà¤¤à¤¿ à¤¸à¥à¤°à¤•à¥à¤·à¤¿à¤¤ à¤¸à¤‚à¤µà¤¾à¤¦
              </p>

              {/* Description */}
              <p className="font-body text-[15px] text-poster-ink mt-3 leading-snug max-w-xs mx-auto">
                Encrypted messages burn upon read or timer expiry. Stored exclusively in volatile
                memory.
              </p>

              <div className="mt-4 pt-3 border-t-2 border-poster-ink flex items-center justify-center gap-3">
                <ChevronMotif size={14} />
                <span className="font-mono text-[10px] uppercase font-bold text-poster-deepred tracking-widest">
                  TOUCH SCREEN OR TYPE BELOW
                </span>
                <ChevronMotif size={14} className="rotate-180" />
              </div>
            </motion.div>
          </div>
        ) : (
          <AnimatePresence>
            {messages.map(msg => {
              const isMe = canonicalHandle(msg.sender) === canonicalHandle(currentUser);

              return (
                <motion.div
                  key={msg.id}
                  layout
                  initial={
                    isMe
                      ? { scale: 0.75, opacity: 0, y: 15 }
                      : { x: -20, opacity: 0, rotate: -3 }
                  }
                  animate={
                    msg.isBurning
                      ? {
                          scale: [1, 0.96, 0.92],
                          opacity: [1, 0.6, 0],
                          filter: 'blur(2px)',
                          y: -8,
                        }
                      : {
                          scale: 1,
                          opacity: 1,
                          x: 0,
                          y: 0,
                          rotate: isMe ? 0.4 : -0.4,
                        }
                  }
                  exit={{
                    opacity: 0,
                    scale: 0.8,
                    height: 0,
                    marginBottom: 0,
                    transition: { duration: 0.35, ease: 'easeInOut' },
                  }}
                  transition={{
                    type: 'spring',
                    stiffness: 350,
                    damping: 22,
                  }}
                  className={`flex flex-col relative ${isMe ? 'items-end' : 'items-start'}`}
                >
                  {/* Floating ash particles when burning */}
                  {msg.isBurning && (
                    <div className="absolute inset-0 pointer-events-none z-20 overflow-visible">
                      {Array.from({ length: 8 }).map((_, pi) => (
                        <motion.div
                          key={pi}
                          initial={{ opacity: 1, y: 0, x: 0 }}
                          animate={{
                            opacity: 0,
                            y: -(20 + pi * 7),
                            x: (pi % 2 === 0 ? 1 : -1) * (10 + pi * 4),
                          }}
                          transition={{ duration: 0.4, ease: 'easeOut' }}
                          className="absolute w-1.5 h-1.5 bg-poster-ink rounded-full"
                          style={{
                            left: `${20 + (pi * 10)}%`,
                            top: '40%',
                          }}
                        />
                      ))}
                    </div>
                  )}

                  {/* Organic Speech Bubble Shape */}
                  <div
                    className={`max-w-lg border-2 border-poster-ink shadow-hard p-3.5 sm:p-4 relative transition-colors ${
                      isMe
                        ? 'bg-poster-mustard rounded-[24px_6px_24px_24px]'
                        : 'bg-white rounded-[6px_24px_24px_24px]'
                    } ${msg.isBurning ? 'border-poster-red bg-poster-red/20' : ''}`}
                  >
                    {/* Header */}
                    <div className="flex items-center justify-between gap-4 pb-1.5 mb-2 border-b border-poster-ink">
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-xs font-bold uppercase text-poster-ink tracking-wide">
                          {isMe ? 'YOU' : msg.sender}
                        </span>
                        {isMe && msg.status && (
                          <span className={`font-mono text-[10px] font-bold px-1 rounded-sm ${
                            msg.status === 'READ' ? 'text-poster-green bg-poster-green/10' :
                            msg.status === 'DELIVERED' ? 'text-poster-ink bg-poster-yellow/40' :
                            msg.status === 'FAILED' ? 'text-poster-red bg-poster-red/20' :
                            'text-poster-ink/60'
                          }`}>
                            {msg.status === 'READ' ? '✓✓ READ' :
                             msg.status === 'DELIVERED' ? '✓✓ DELIVERED' :
                             msg.status === 'SENT' ? '✓ SENT' :
                             msg.status === 'FAILED' ? '⚠ FAILED' :
                             '… SENDING'}
                          </span>
                        )}
                      </div>
                      <span className="font-mono text-[10px] text-poster-ink font-semibold">
                        {new Date(msg.timestampMS > 0 ? msg.timestampMS : Date.now()).toLocaleTimeString(
                          [],
                          {
                            hour: '2-digit',
                            minute: '2-digit',
                            second: '2-digit',
                          }
                        )}
                      </span>
                    </div>

                    {/* Message Content: Min 15px, Ink Text, WCAG AA Contrast */}
                    {msg.isDecryptError ? (
                      <div className="p-3 bg-poster-red/15 border-2 border-poster-red rounded-[12px_4px_12px_4px] my-1">
                        <div className="flex items-center gap-1.5 text-poster-deepred font-mono text-xs font-bold uppercase mb-1">
                          <IconLock size={13} />
                          <span>Cannot decrypt (re-key required)</span>
                        </div>
                        <p className="font-mono text-xs text-poster-ink mb-2">
                          Message payload could not be decrypted. Never auto-burned.
                        </p>
                        {onRetryDecrypt && (
                          <button
                            type="button"
                            onClick={() => onRetryDecrypt(msg)}
                            className="poster-btn px-3 py-1 bg-poster-mustard hover:bg-poster-yellow text-poster-ink font-mono text-[11px] font-bold uppercase border border-poster-ink shadow-hard-sm cursor-pointer"
                          >
                            TAP TO RETRY / RE-KEY
                          </button>
                        )}
                      </div>
                    ) : (
                      <p className="font-body text-[15px] sm:text-base text-poster-ink leading-relaxed whitespace-pre-wrap select-text font-normal">
                        {msg.plaintext}
                      </p>
                    )}

                    {/* Segmented Burn Timer Footer */}
                    <div className="flex items-center justify-between gap-4 mt-3 pt-2 border-t border-poster-ink">
                      {!msg.isDecryptError ? (
                        <CountdownTimer
                          timestampMS={msg.timestampMS}
                          ttlMS={msg.ttlMS}
                          onExpire={() => onBurnMessage(msg.id)}
                        />
                      ) : (
                        <span className="font-mono text-[10px] font-bold uppercase text-poster-deepred">
                          PERSISTED UNTIL TTL / NEVER AUTO-BURNED
                        </span>
                      )}

                      <button
                        type="button"
                        onClick={() => onBurnMessage(msg.id)}
                        title="BURN MESSAGE NOW"
                        className="poster-btn p-1 bg-white hover:bg-poster-red hover:text-white text-poster-ink transition-colors rounded-[4px]"
                      >
                        <IconTrash size={13} />
                      </button>
                    </div>
                  </div>
                </motion.div>
              );
            })}
          </AnimatePresence>
        )}
        {/* Peer Typing Indicator in Feed */}
        {isPeerTyping && (
          <div className="flex items-center gap-2 p-2 bg-white/80 border border-poster-ink rounded-[8px_2px_8px_2px] w-fit shadow-hard-sm">
            <PetalTypingIndicator />
            <span className="font-mono text-xs font-bold text-poster-ink uppercase">
              {currentContact} is typing…
            </span>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input Desk with Petal Ready Indicator */}
      <form
        onSubmit={handleSend}
        className="p-3 sm:p-4 bg-poster-cream border-t-2 border-poster-ink flex flex-col sm:flex-row gap-2.5 items-stretch sm:items-center z-10 relative"
      >
        {/* Send Confetti Burst */}
        {showSendBurst && <SendConfettiBurst />}

        <div className="hidden sm:block shrink-0">
          <PetalTypingIndicator />
        </div>

        <div className="relative flex-1">
          <input
            type="text"
            placeholder="Type an ephemeral message…"
            value={inputText}
            onChange={handleInputChange}
            className="w-full bg-white border-2 border-poster-ink text-poster-ink font-body text-[15px] px-3.5 py-2.5 shadow-hard-sm focus:outline-none focus:bg-poster-cream rounded-[16px_4px_16px_4px] placeholder:text-poster-ink/50"
          />
        </div>

        <motion.button
          whileHover={{ scale: 1.03 }}
          whileTap={{ scale: 0.94 }}
          type="submit"
          className="poster-btn px-5 sm:px-6 py-2.5 bg-poster-mustard hover:bg-poster-yellow text-poster-ink font-bungee text-sm tracking-wide rounded-[12px_4px_12px_4px] flex items-center justify-center gap-2 shrink-0 uppercase cursor-pointer"
        >
          <IconSend size={15} />
          <span>SEND</span>
        </motion.button>
      </form>
    </div>
  );
};
