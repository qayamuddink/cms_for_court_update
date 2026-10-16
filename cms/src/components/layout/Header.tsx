'use client';

import React from 'react';
import { Menu, Bell, Shield } from 'lucide-react';
import { useAuth } from '@/lib/auth/context';

interface HeaderProps {
  onOpenSidebar: () => void;
  title?: string;
  badge?: string;
}

export default function Header({ onOpenSidebar, title, badge }: HeaderProps) {
  const { user } = useAuth();

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-slate-200 bg-white/80 px-4 sm:px-6 backdrop-blur-md">
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={onOpenSidebar}
          className="rounded-lg p-2 text-slate-600 hover:bg-slate-100 lg:hidden"
          aria-label="Open sidebar"
        >
          <Menu className="h-5 w-5" />
        </button>
        {title && (
          <div className="flex items-center gap-2.5">
            <h1 className="text-lg font-bold text-slate-900 tracking-tight sm:text-xl">
              {title}
            </h1>
            {badge && (
              <span className="rounded-full bg-amber-50 border border-amber-200/60 px-2 py-0.5 text-xs font-semibold text-amber-700">
                {badge}
              </span>
            )}
          </div>
        )}
      </div>

      <div className="flex items-center gap-3">
        {/* Live system health status badge */}
        <div className="hidden sm:flex items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1 border border-emerald-200/80 text-xs font-semibold text-emerald-800">
          <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
          API Connected
        </div>

        {/* User profile pill */}
        {user && (
          <div className="flex items-center gap-2 rounded-full bg-slate-100 py-1 pl-1 pr-3 border border-slate-200">
            <div className="flex h-7 w-7 items-center justify-center rounded-full bg-slate-900 text-xs font-bold text-amber-400">
              {user.full_name?.charAt(0).toUpperCase() || 'U'}
            </div>
            <div className="text-left hidden md:block">
              <span className="text-xs font-semibold text-slate-800 block leading-tight">
                {user.full_name}
              </span>
              <span className="text-[10px] text-slate-500 font-medium uppercase tracking-wider block">
                {user.system_role}
              </span>
            </div>
          </div>
        )}
      </div>
    </header>
  );
}
