import React, { useState } from 'react';
import { CountdownTimer } from './CountdownTimer.tsx';
import { canonicalHandle } from '../crypto/ratchet.ts';
import { LotusMotif, ChevronMotif, IconSend, IconTrash, IconLock } from './PosterMotifs.tsx';

export interface DisplayMessage {
  id: string;
  sender: string;
  recipient: string;
  plaintext: string;
  timestampMS: number;
  ttlMS: number;
  isBurning?: boolean;
}

interface ChatFeedProps {
  currentContact: string;
  currentUser: string;
  messages: DisplayMessage[];
  onSendMessage: (text: string, ttlMS: number) => void;
  onBurnMessage: (id: string) => void;
}

export const ChatFeed: React.FC<ChatFeedProps> = ({
  currentContact,
  currentUser,
  messages,
  onSendMessage,
  onBurnMessage,
}) => {
  const [inputText, setInputText] = useState('');
  const [ttlMS, setTtlMS] = useState(10000); // 10s default

  const handleSend = (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputText.trim()) return;
    onSendMessage(inputText.trim(), ttlMS);
    setInputText('');
  };

  const contactInitial = currentContact.replace('@', '').charAt(0).toUpperCase();

  return (
    <div className="flex flex-col h-full bg-poster-cream border-2 border-poster-ink shadow-hard rounded-none overflow-hidden relative">
      {/* Top Header */}
      <div className="flex items-center justify-between px-4 sm:px-6 py-3 border-b-2 border-poster-ink bg-poster-cream z-10">
        <div className="flex items-center gap-3">
          {/* Square Avatar with 2px ink border */}
          <div className="w-10 h-10 bg-poster-yellow border-2 border-poster-ink flex items-center justify-center font-bungee text-poster-ink text-base shadow-hard-sm">
            {contactInitial}
          </div>

          <div>
            <div className="flex items-center gap-2">
              <h2 className="font-bungee text-base sm:text-lg text-poster-ink uppercase tracking-wide">
                {currentContact}
              </h2>
              <span className="font-mono text-[11px] font-bold px-1.5 py-0.5 bg-poster-green text-poster-cream border border-poster-ink uppercase">
                ACTIVE
              </span>
            </div>
            <div className="font-mono text-[11px] uppercase text-poster-deepred font-bold flex items-center gap-1.5">
              <IconLock size={11} />
              <span>PAIRWISE HKDF / ZERO-KNOWLEDGE E2EE</span>
            </div>
          </div>
        </div>

        {/* Flat Burn TTL Selector */}
        <div className="flex items-center gap-2">
          <label htmlFor="ttl-select" className="font-mono text-xs uppercase font-bold text-poster-ink hidden sm:inline">
            BURN TIMER:
          </label>
          <select
            id="ttl-select"
            value={ttlMS}
            onChange={e => setTtlMS(Number(e.target.value))}
            className="font-mono text-xs font-bold uppercase bg-white border-2 border-poster-ink px-2.5 py-1.5 shadow-hard-sm focus:outline-none cursor-pointer"
          >
            <option value={5000}>5 SECONDS</option>
            <option value={10000}>10 SECONDS</option>
            <option value={30000}>30 SECONDS</option>
            <option value={60000}>60 SECONDS</option>
          </select>
        </div>
      </div>

      {/* Message Feed Area */}
      <div className="flex-1 overflow-y-auto p-4 sm:p-6 space-y-4 bg-poster-cream relative">
        {messages.length === 0 ? (
          /* Empty Chat State ONLY: Symmetrical truck-art style frame with cobalt cutout, original illustration, Bungee GUPSHUP headline with yellow-to-orange split, small Devanagari subtitle */
          <div className="h-full flex flex-col items-center justify-center p-4">
            <div className="w-full max-w-md bg-poster-cream border-2 border-poster-ink shadow-hard p-6 text-center relative">
              {/* Corner Lotus Motifs */}
              <div className="absolute top-2 left-2">
                <LotusMotif size={22} />
              </div>
              <div className="absolute top-2 right-2 scale-x-[-1]">
                <LotusMotif size={22} />
              </div>
              <div className="absolute bottom-2 left-2 rotate-180 scale-x-[-1]">
                <LotusMotif size={22} />
              </div>
              <div className="absolute bottom-2 right-2 rotate-180">
                <LotusMotif size={22} />
              </div>

              {/* Symmetrical Arch Frame with Cobalt Cutout */}
              <div className="w-48 h-36 mx-auto mb-4 bg-poster-cobalt border-2 border-poster-ink shadow-hard-sm relative flex flex-col items-center justify-center overflow-hidden p-2">
                {/* Arch outline in cream/yellow */}
                <div className="absolute inset-1.5 border border-poster-yellow pointer-events-none" />

                {/* Original Folk Graphic: Symmetrical Sunburst & Lotus Illustration */}
                <svg width="120" height="80" viewBox="0 0 120 80" fill="none" xmlns="http://www.w3.org/2000/svg">
                  {/* Sunburst rays */}
                  <line x1="60" y1="40" x2="60" y2="8" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
                  <line x1="60" y1="40" x2="25" y2="18" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
                  <line x1="60" y1="40" x2="95" y2="18" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
                  <line x1="60" y1="40" x2="10" y2="40" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
                  <line x1="60" y1="40" x2="110" y2="40" stroke="#F7D63C" strokeWidth="2" strokeDasharray="3 3" />
                  
                  {/* Symmetrical geometric folk birds / wings */}
                  <path d="M60 42C48 30 30 35 20 48C34 50 48 48 60 42Z" fill="#F4E7C8" stroke="#2A0A08" strokeWidth="1.5" />
                  <path d="M60 42C72 30 90 35 100 48C86 50 72 48 60 42Z" fill="#F4E7C8" stroke="#2A0A08" strokeWidth="1.5" />

                  {/* Center folk lotus core */}
                  <circle cx="60" cy="42" r="10" fill="#F2A33A" stroke="#2A0A08" strokeWidth="1.5" />
                  <circle cx="60" cy="42" r="4" fill="#C8141A" />
                  
                  {/* Base waves */}
                  <path d="M20 62C30 58 40 66 50 62C60 58 70 66 80 62C90 58 100 66 110 62" stroke="#F7D63C" strokeWidth="2" />
                </svg>

                <div className="font-mono text-[9px] uppercase tracking-widest text-poster-cream font-bold mt-1">
                  NO. 01 / WIRE
                </div>
              </div>

              {/* Chunky 3D Bungee Headline with Yellow-to-Mustard Hard Split */}
              <h3 className="font-bungee text-3xl sm:text-4xl poster-split-title tracking-wider leading-none">
                GUPSHUP
              </h3>

              {/* Small Devanagari Subtitle */}
              <p className="font-yatra text-base text-poster-red mt-1 font-bold">
                शून्य-स्मृति सुरक्षित संवाद
              </p>

              {/* English Description */}
              <p className="font-body text-[15px] text-poster-ink mt-3 leading-snug max-w-xs mx-auto">
                Encrypted messages burn upon read or timer expiry. Stored exclusively in volatile memory.
              </p>

              <div className="mt-4 pt-3 border-t border-poster-ink flex items-center justify-center gap-3">
                <ChevronMotif size={14} />
                <span className="font-mono text-[10px] uppercase font-bold text-poster-deepred tracking-widest">
                  READY FOR DISPATCH
                </span>
                <ChevronMotif size={14} className="rotate-180" />
              </div>
            </div>
          </div>
        ) : (
          messages.map(msg => {
            const isMe = canonicalHandle(msg.sender) === canonicalHandle(currentUser);
            return (
              <div
                key={msg.id}
                className={`flex flex-col ${isMe ? 'items-end' : 'items-start'} ${
                  msg.isBurning ? 'opacity-30 translate-y-1 transition-all duration-300' : ''
                }`}
              >
                {/* Sharp rectangular message block with 2px ink border and hard shadow */}
                <div
                  className={`max-w-lg border-2 border-poster-ink shadow-hard p-3.5 sm:p-4 rounded-none relative ${
                    isMe ? 'bg-poster-mustard' : 'bg-poster-cream'
                  }`}
                >
                  {/* Sender Header */}
                  <div className="flex items-center justify-between gap-4 pb-1.5 mb-2 border-b border-poster-ink">
                    <span className="font-mono text-xs font-bold uppercase text-poster-ink tracking-wide">
                      {isMe ? 'YOU' : msg.sender}
                    </span>
                    <span className="font-mono text-[10px] text-poster-ink font-semibold">
                      {new Date(msg.timestampMS > 0 ? msg.timestampMS : Date.now()).toLocaleTimeString([], {
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                      })}
                    </span>
                  </div>

                  {/* Message Content: Min 15px, Ink Text, WCAG AA Contrast */}
                  <p className="font-body text-[15px] sm:text-base text-poster-ink leading-relaxed whitespace-pre-wrap select-text font-normal">
                    {msg.plaintext}
                  </p>

                  {/* Segmented Burn Timer Footer */}
                  <div className="flex items-center justify-between gap-4 mt-3 pt-2 border-t border-poster-ink">
                    <CountdownTimer
                      timestampMS={msg.timestampMS}
                      ttlMS={msg.ttlMS}
                      onExpire={() => onBurnMessage(msg.id)}
                    />

                    <button
                      type="button"
                      onClick={() => onBurnMessage(msg.id)}
                      title="BURN MESSAGE NOW"
                      className="poster-btn p-1 bg-white hover:bg-poster-red hover:text-white text-poster-ink transition-colors"
                    >
                      <IconTrash size={13} />
                    </button>
                  </div>
                </div>
              </div>
            );
          })
        )}
      </div>

      {/* Input Desk */}
      <form
        onSubmit={handleSend}
        className="p-3 sm:p-4 bg-poster-cream border-t-2 border-poster-ink flex gap-2.5 items-center z-10"
      >
        <div className="relative flex-1">
          <input
            type="text"
            placeholder="Type a message…"
            value={inputText}
            onChange={e => setInputText(e.target.value)}
            className="w-full bg-white border-2 border-poster-ink text-poster-ink font-body text-[15px] px-3.5 py-2.5 shadow-hard-sm focus:outline-none focus:bg-poster-cream rounded-none placeholder:text-poster-ink/50"
          />
        </div>

        <button
          type="submit"
          className="poster-btn px-5 sm:px-6 py-2.5 bg-poster-mustard hover:bg-poster-yellow text-poster-ink font-bungee text-sm tracking-wide rounded-none flex items-center gap-2 shrink-0 uppercase"
        >
          <IconSend size={15} />
          <span>SEND</span>
        </button>
      </form>
    </div>
  );
};
