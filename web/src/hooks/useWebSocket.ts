import { useEffect, useRef, useState, useCallback } from 'react';
import { WebEnvelope } from '../crypto/ratchet.ts';

export interface PresenceInfo {
  userHandle: string;
  zoneCode: string;
  displayZone: string;
  buildingName?: string;
  locationEnabled: boolean;
}

export function useWebSocket(token: string | null) {
  const wsRef = useRef<WebSocket | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [incomingEnvelope, setIncomingEnvelope] = useState<WebEnvelope | null>(null);
  const [burnedMessageId, setBurnedMessageId] = useState<string | null>(null);
  const [peerPresence, setPeerPresence] = useState<PresenceInfo | null>(null);

  useEffect(() => {
    if (!token) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/ws/v1/stream?token=${encodeURIComponent(token)}`;

    let ws: WebSocket;
    let heartbeatTimer: any;

    const connect = () => {
      ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        setIsConnected(true);
        heartbeatTimer = setInterval(() => {
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ op: 'HEARTBEAT' }));
          }
        }, 25000);
      };

      ws.onmessage = (event) => {
        try {
          const frame = JSON.parse(event.data);
          if (frame.op === 'DELIVER_ENVELOPE' && frame.envelope) {
            setIncomingEnvelope(frame.envelope);
          } else if (frame.op === 'ACK_READ' && frame.ack_message_id) {
            setBurnedMessageId(frame.ack_message_id);
          } else if (frame.op === 'ZONE_PRESENCE' && frame.presence) {
            setPeerPresence({
              userHandle: frame.presence.user_handle,
              zoneCode: frame.presence.zone_code,
              displayZone: frame.presence.display_zone,
              buildingName: frame.presence.building_name,
              locationEnabled: frame.presence.location_enabled,
            });
          }
        } catch (e) {
          console.error('[WS Parse Error]', e);
        }
      };

      ws.onclose = () => {
        setIsConnected(false);
        clearInterval(heartbeatTimer);
        // Reconnect after 3 seconds
        setTimeout(connect, 3000);
      };

      ws.onerror = () => {
        ws.close();
      };
    };

    connect();

    return () => {
      clearInterval(heartbeatTimer);
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [token]);

  const sendEnvelope = useCallback((envelope: WebEnvelope) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        op: 'SEND_ENVELOPE',
        envelope: envelope,
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
    sendEnvelope,
    ackRead,
    updatePresence,
  };
}
