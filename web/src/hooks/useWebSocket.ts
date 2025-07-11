import { useEffect, useRef } from 'react';

export function useWebSocket(url: string, onMessage: (msg: MessageEvent) => void) {
  const ws = useRef<WebSocket | null>(null);

  useEffect(() => {
    ws.current = new WebSocket(url);
    ws.current.onmessage = onMessage;
    return () => {
      ws.current?.close();
    };
  }, [url, onMessage]);

  return ws.current;
}
