import { useEffect, useRef } from 'react';

export function usePolling(callback: () => void | Promise<void>, delayMs = 30_000, enabled = true) {
  const callbackRef = useRef(callback);
  useEffect(() => { callbackRef.current = callback; }, [callback]);
  useEffect(() => {
    if (!enabled || delayMs < 1_000) return;
    const timer = window.setInterval(() => void callbackRef.current(), delayMs);
    return () => window.clearInterval(timer);
  }, [delayMs, enabled]);
}
