'use client';

import React, { createContext, useContext, useEffect, useState } from 'react';
import apiClient, { getStoredTokens, setAuthTokens, setUnauthorizedHandler } from '../api/client';
import { useRouter, usePathname } from 'next/navigation';

export interface User {
  id: string;
  email: string;
  full_name: string;
  display_role: string;
  system_role: 'reader' | 'reporter' | 'editor' | 'admin';
  status: string;
}

interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<{ success: boolean; error?: string }>;
  logout: () => Promise<void>;
  hasRole: (...roles: ('reader' | 'reporter' | 'editor' | 'admin')[]) => boolean;
  isStaff: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const router = useRouter();
  const pathname = usePathname();

  const fetchCurrentUser = async () => {
    try {
      const { data, error } = await apiClient.GET('/v1/me');
      if (data && 'id' in data) {
        setUser(data as unknown as User);
      } else if (error) {
        setUser(null);
        setAuthTokens(null, null);
      }
    } catch {
      setUser(null);
      setAuthTokens(null, null);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    const { accessToken } = getStoredTokens();
    if (accessToken) {
      fetchCurrentUser();
    } else {
      setIsLoading(false);
    }

    setUnauthorizedHandler(() => {
      setUser(null);
      if (pathname !== '/login') {
        router.push('/login');
      }
    });
  }, []);

  const login = async (email: string, password: string) => {
    setIsLoading(true);
    try {
      const { data, error, response } = await apiClient.POST('/v1/auth/login', {
        body: { email, password },
      });

      if (!response.ok || !data) {
        setIsLoading(false);
        const errMsg = (error as any)?.message || 'Invalid email or password';
        return { success: false, error: errMsg };
      }

      setAuthTokens(data.access_token, data.refresh_token);
      setUser(data.user as User);
      setIsLoading(false);
      return { success: true };
    } catch (err: any) {
      setIsLoading(false);
      return { success: false, error: err.message || 'Connection failed' };
    }
  };

  const logout = async () => {
    const { refreshToken } = getStoredTokens();
    if (refreshToken) {
      try {
        await apiClient.POST('/v1/auth/logout', {
          body: { refresh_token: refreshToken },
        });
      } catch {
        // Ignore network errors on logout
      }
    }
    setAuthTokens(null, null);
    setUser(null);
    router.push('/login');
  };

  const hasRole = (...roles: ('reader' | 'reporter' | 'editor' | 'admin')[]) => {
    if (!user) return false;
    return roles.includes(user.system_role);
  };

  const isStaff = !!user && ['reporter', 'editor', 'admin'].includes(user.system_role);

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        isAuthenticated: !!user,
        login,
        logout,
        hasRole,
        isStaff,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
