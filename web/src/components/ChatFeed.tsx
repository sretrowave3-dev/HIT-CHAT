import React, { useState } from 'react';
import { Send, Trash2, Clock, Lock } from 'lucide-react';
import { CountdownTimer } from './CountdownTimer.tsx';
import { canonicalHandle } from '../crypto/ratchet.ts';

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

  return (
    <div className="flex flex-col h-full bg-cyber-800/60 border border-cyber-700 rounded-2xl overflow-hidden shadow-2xl">
      {/* Feed Header */}
      <div className="flex items-center justify-between px-6 py-4 border-b border-cyber-700 bg-cyber-800">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-full bg-emerald-500/20 border border-emerald-500/40 flex items-center justify-center font-bold text-emerald-400">
            {currentContact.replace('@', '').charAt(0).toUpperCase()}
          </div>
          <div>
            <h2 className="text-base font-bold text-gray-100 flex items-center gap-2">
              <span>{currentContact}</span>
              <Lock className="w-3.5 h-3.5 text-emerald-400" />
            </h2>
            <p className="text-xs text-emerald-400 font-mono">E2EE Double Ratchet Active</p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Clock className="w-4 h-4 text-gray-400" />
          <select
            value={ttlMS}
            onChange={e => setTtlMS(Number(e.target.value))}
            className="bg-cyber-900 border border-gray-700 text-xs text-gray-300 rounded-lg px-2.5 py-1.5 focus:outline-none focus:border-emerald-500"
          >
            <option value={5000}>Burn: 5 seconds</option>
            <option value={10000}>Burn: 10 seconds</option>
            <option value={30000}>Burn: 30 seconds</option>
            <option value={60000}>Burn: 60 seconds</option>
          </select>
        </div>
      </div>

      {/* Message Feed */}
      <div className="flex-1 overflow-y-auto p-6 space-y-4">
        {messages.length === 0 ? (
          <div className="h-full flex flex-col items-center justify-center text-center text-gray-500 p-8">
            <Lock className="w-12 h-12 text-cyber-600 mb-3 stroke-[1.5]" />
            <p className="text-sm font-medium">Ephemeral Secure Session Established</p>
            <p className="text-xs text-gray-500 mt-1 max-w-sm">
              All messages exchanged with {currentContact} are protected by zero-knowledge Curve25519 ratchets and self-destruct upon expiration or read acknowledgement.
            </p>
          </div>
        ) : (
          messages.map(msg => {
            const isMe = canonicalHandle(msg.sender) === canonicalHandle(currentUser);
            return (
              <div
                key={msg.id}
                className={`flex flex-col ${isMe ? 'items-end' : 'items-start'} ${
                  msg.isBurning ? 'animate-burn-fade pointer-events-none' : ''
                }`}
              >
                <div
                  className={`max-w-md rounded-2xl px-4 py-3 shadow-md relative group border ${
                    isMe
                      ? 'bg-emerald-950/40 border-emerald-500/30 text-emerald-100 rounded-br-sm'
                      : 'bg-cyber-700/60 border-cyber-600 text-gray-100 rounded-bl-sm'
                  }`}
                >
                  <p className="text-sm leading-relaxed whitespace-pre-wrap select-text">
                    {msg.plaintext}
                  </p>

                  <div className="flex items-center justify-between gap-4 mt-2 pt-2 border-t border-gray-700/40">
                    <CountdownTimer
                      timestampMS={msg.timestampMS}
                      ttlMS={msg.ttlMS}
                      onExpire={() => onBurnMessage(msg.id)}
                    />

                    <button
                      onClick={() => onBurnMessage(msg.id)}
                      title="Immediate Burn-on-Read"
                      className="opacity-0 group-hover:opacity-100 transition-opacity p-1 text-gray-400 hover:text-red-400"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
                <span className="text-[10px] text-gray-500 mt-1 px-1">
                  {isMe ? 'You' : msg.sender} • {new Date(msg.timestampMS > 0 ? msg.timestampMS : Date.now()).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                </span>
              </div>
            );
          })
        )}
      </div>

      {/* Input Form */}
      <form onSubmit={handleSend} className="p-4 bg-cyber-800 border-t border-cyber-700 flex gap-3">
        <input
          type="text"
          placeholder={`Send ephemeral message to ${currentContact}...`}
          value={inputText}
          onChange={e => setInputText(e.target.value)}
          className="flex-1 bg-cyber-900 border border-gray-700 rounded-xl px-4 py-3 text-sm text-gray-100 placeholder-gray-500 focus:outline-none focus:border-emerald-500"
        />
        <button
          type="submit"
          className="px-5 py-3 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold rounded-xl shadow-lg shadow-emerald-900/30 transition flex items-center gap-2"
        >
          <Send className="w-4 h-4" />
          <span className="hidden sm:inline">Send</span>
        </button>
      </form>
    </div>
  );
};
