import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { clearSession, getStoredSession, saveSession } from '../api/client';
import { login } from '../api/auth';
import type { UserSession } from '../types/domain';

interface AuthContextValue {
  session: UserSession | null;
  loading: boolean;
  authenticated: boolean;
  switchAccount: (username: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<UserSession | null>(() => getStoredSession());
  const [loading, setLoading] = useState(() => !getStoredSession());
  const booted = useRef(false);

  const switchAccount = useCallback(async (username: string) => {
    setLoading(true);
    try {
      const next = await login(username, 'Admin123!');
      saveSession(next);
      setSession(next);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (booted.current) return;
    booted.current = true;
    if (!getStoredSession()) void switchAccount('admin');
  }, [switchAccount]);

  const logout = useCallback(async () => {
    clearSession();
    setSession(null);
    await switchAccount('admin');
  }, [switchAccount]);

  const value = useMemo(() => ({ session, loading, authenticated: Boolean(session?.token), switchAccount, logout }), [session, loading, switchAccount, logout]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth must be used inside AuthProvider');
  return value;
}
