import { useEffect, useRef, useState, useCallback } from 'react';
import { WebEnvelope } from '../crypto/ratchet.ts';

export interface PresenceInfo {
  userHandle: string;
  zoneCode: string;
  displayZone: string;
  buildingName?: string;
  locationEnabled: boolean;
}

export interface UseWebSocketOptions {
  onEnvelope?: (env: WebEnvelope) => void;
  onBurn?: (messageId: string) => void;
  onAckDelivered?: (messageId: string) => void;
  onAckRead?: (messageId: string) => void;
  onNack?: (messageId: string, reason: string) => void;
  onPresence?: (presence: PresenceInfo) => void;
  onTyping?: (senderId: string, isTyping: boolean) => void;
}

export function useWebSocket(token: string | null, options?: UseWebSocketOptions) {
  const wsRef = useRef<WebSocket | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [incomingEnvelope, setIncomingEnvelope] = useState<WebEnvelope | null>(null);
  const [burnedMessageId, setBurnedMessageId] = useState<string | null>(null);
  const [peerPresence, setPeerPresence] = useState<PresenceInfo | null>(null);
  const [remoteTyping, setRemoteTyping] = useState<{ [sender: string]: boolean }>({});
  
  // Offline outbox queue
  const outboxQueueRef = useRef<WebEnvelope[]>([]);
  const retryCountRef = useRef(0);
  const optionsRef = useRef(options);
  optionsRef.current = options;

  const flushOutbox = useCallback(() => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    while (outboxQueueRef.current.length > 0) {
      const env = outboxQueueRef.current.shift();
      if (env) {
        wsRef.current.send(JSON.stringify({
          op: 'SEND_ENVELOPE',
          envelope: env,
        }));
      }
    }
  }, []);

  useEffect(() => {
    if (!token) return;

    let isMounted = true;
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/ws/v1/stream?token=${encodeURIComponent(token)}`;

    let ws: WebSocket;
    let heartbeatTimer: any;
    let reconnectTimer: any;

    const scheduleReconnect = () => {
      if (!isMounted) return;
      setIsConnected(false);
      clearInterval(heartbeatTimer);
      const retry = retryCountRef.current;
      retryCountRef.current += 1;
      // Exponential backoff with jitter (max 15s)
      const delay = Math.min(1000 * Math.pow(1.5, retry) + Math.random() * 500, 15000);
      clearTimeout(reconnectTimer);
      reconnectTimer = setTimeout(() => {
        if (isMounted) connect();
      }, delay);
    };

    const connect = () => {
      if (!isMounted) return;
      try {
        ws = new WebSocket(wsUrl);
        wsRef.current = ws;
      } catch (err) {
        console.warn('[WS Connect Error]', err);
        scheduleReconnect();
        return;
      }

      ws.onopen = () => {
        if (!isMounted) {
          ws.close();
          return;
        }
        setIsConnected(true);
        retryCountRef.current = 0; // reset backoff
        flushOutbox();

        heartbeatTimer = setInterval(() => {
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ op: 'HEARTBEAT' }));
          }
        }, 25000);
      };

      ws.onmessage = (event) => {
        if (!isMounted) return;
        try {
          const frame = JSON.parse(event.data);
          switch (frame.op) {
            case 'DELIVER_ENVELOPE':
              if (frame.envelope) {
                setIncomingEnvelope(frame.envelope);
                optionsRef.current?.onEnvelope?.(frame.envelope);
              }
              break;

            case 'ACK_DELIVERED':
              if (frame.ack_message_id) {
                optionsRef.current?.onAckDelivered?.(frame.ack_message_id);
              }
              break;

            case 'ACK_READ':
              if (frame.ack_message_id) {
                setBurnedMessageId(frame.ack_message_id);
                optionsRef.current?.onBurn?.(frame.ack_message_id);
                optionsRef.current?.onAckRead?.(frame.ack_message_id);
              }
              break;

            case 'NACK':
              if (frame.ack_message_id) {
                optionsRef.current?.onNack?.(frame.ack_message_id, frame.nack_reason || 'NACK');
              }
              break;

            case 'TYPING':
              if (frame.typing) {
                setRemoteTyping(prev => ({
                  ...prev,
                  [frame.typing.sender_id]: frame.typing.is_typing,
                }));
                optionsRef.current?.onTyping?.(frame.typing.sender_id, frame.typing.is_typing);
              }
              break;

            case 'ZONE_PRESENCE':
              if (frame.presence) {
                const pInfo: PresenceInfo = {
                  userHandle: frame.presence.user_handle,
                  zoneCode: frame.presence.zone_code,
                  displayZone: frame.presence.display_zone,
                  buildingName: frame.presence.building_name,
                  locationEnabled: frame.presence.location_enabled,
                };
                setPeerPresence(pInfo);
                optionsRef.current?.onPresence?.(pInfo);
              }
              break;

            default:
              break;
          }
        } catch (e) {
          console.error('[WS Parse Error]', e);
        }
      };

      ws.onclose = scheduleReconnect;
      ws.onerror = () => {
        try {
          ws.close();
        } catch {}
      };
    };

    connect();

    return () => {
      isMounted = false;
      clearInterval(heartbeatTimer);
      clearTimeout(reconnectTimer);
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [token, flushOutbox]);

  const sendEnvelope = useCallback((envelope: WebEnvelope) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        op: 'SEND_ENVELOPE',
        envelope: envelope,
      }));
    } else {
      // Buffer in offline outbox
      outboxQueueRef.current.push(envelope);
    }
  }, []);

  const ackDelivered = useCallback((messageId: string) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        op: 'ACK_DELIVERED',
        ack_message_id: messageId,
      }));
    }
  }, []);

  const ackRead = useCallback((messageId: string) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        op: 'ACK_READ',
        ack_message_id: messageId,
      }));
    }
  }, []);

  const sendNack = useCallback((messageId: string, reason: string) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        op: 'NACK',
        ack_message_id: messageId,
        nack_reason: reason,
      }));
    }
  }, []);

  const sendTyping = useCallback((recipientId: string, isTyping: boolean) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        op: 'TYPING',
        typing: {
          recipient_id: recipientId,
          is_typing: isTyping,
        },
      }));
    }
  }, []);

  const updatePresence = useCallback((presence: { bssidHash: string; locationEnabled: boolean }) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        op: 'UPDATE_PRESENCE',
        presence: {
          bssid_hash: presence.bssidHash,
          location_enabled: presence.locationEnabled,
        },
      }));
    }
  }, []);

  return {
    isConnected,
    incomingEnvelope,
    burnedMessageId,
    peerPresence,
    remoteTyping,
    outboxLength: outboxQueueRef.current.length,
    sendEnvelope,
    ackDelivered,
    ackRead,
    sendNack,
    sendTyping,
    updatePresence,
  };
}
