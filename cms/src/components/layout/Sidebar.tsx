'use client';

import React from 'react';
import Link from 'next/navigation';
import { usePathname } from 'next/navigation';
import { useAuth } from '@/lib/auth/context';
import {
  Radio,
  LayoutDashboard,
  FileText,
  Clock,
  BookOpen,
  Briefcase,
  Users,
  LogOut,
  X,
  Scale,
} from 'lucide-react';

interface SidebarProps {
  isOpen: boolean;
  onClose: () => void;
}

export default function Sidebar({ isOpen, onClose }: SidebarProps) {
  const pathname = usePathname();
  const { user, logout, hasRole } = useAuth();

  const navigation = [
    {
      name: 'Dashboard',
      href: '/dashboard',
      icon: LayoutDashboard,
      show: true,
    },
    {
      name: 'Live Court Desk',
      href: '/desk',
      icon: Radio,
      badge: 'LIVE',
      show: hasRole('reporter', 'editor', 'admin'),
    },
    {
      name: 'Articles',
      href: '/articles',
      icon: FileText,
      show: hasRole('reporter', 'editor', 'admin'),
    },
    {
      name: 'Review Queue',
      href: '/reviews',
      icon: Clock,
      show: hasRole('editor', 'admin'),
    },
    {
      name: 'Judgments',
      href: '/judgments',
      icon: BookOpen,
      show: hasRole('editor', 'admin'),
    },
    {
      name: 'Court Cases',
      href: '/cases',
      icon: Briefcase,
      show: hasRole('editor', 'admin'),
    },
    {
      name: 'Staff & Plans',
      href: '/users',
      icon: Users,
      show: hasRole('admin'),
    },
  ];

  return (
    <>
      {/* Mobile backdrop */}
      {isOpen && (
        <div
          className="fixed inset-0 z-40 bg-slate-900/60 backdrop-blur-sm lg:hidden transition-opacity"
          onClick={onClose}
          aria-hidden="true"
        />
      )}

      {/* Sidebar container */}
      <aside
        className={`fixed top-0 bottom-0 left-0 z-50 flex w-72 flex-col bg-slate-900 border-r border-slate-800 transition-transform duration-300 ease-in-out lg:translate-x-0 ${
          isOpen ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        {/* Brand header */}
        <div className="flex h-16 items-center justify-between px-6 bg-slate-950/60 border-b border-slate-800">
          <a href="/dashboard" className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-tr from-amber-600 to-amber-400 text-slate-950 shadow-md shadow-amber-500/20">
              <Scale className="h-6 w-6 stroke-[2.2]" />
            </div>
            <div>
              <div className="font-bold tracking-tight text-white flex items-center gap-1.5 text-base">
                LiveLaw <span className="text-xs uppercase tracking-wider px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-300 font-semibold border border-amber-500/30">CMS</span>
              </div>
              <div className="text-[11px] font-medium text-slate-400">Editorial &amp; Court Desk</div>
            </div>
          </a>
          <button
            type="button"
            className="lg:hidden rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-white"
            onClick={onClose}
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Navigation list */}
        <nav className="flex-1 space-y-1.5 px-3 py-4 overflow-y-auto">
          {navigation
            .filter((item) => item.show)
            .map((item) => {
              const isActive = pathname === item.href;
              const Icon = item.icon;
              return (
                <a
                  key={item.name}
                  href={item.href}
                  onClick={onClose}
                  className={`group flex items-center justify-between rounded-xl px-3.5 py-2.5 text-sm font-medium transition-all ${
                    isActive
                      ? 'bg-amber-500 text-slate-950 font-semibold shadow-md shadow-amber-500/15'
                      : 'text-slate-300 hover:bg-slate-800/80 hover:text-white'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <Icon
                      className={`h-5 w-5 shrink-0 transition-colors ${
                        isActive ? 'text-slate-950' : 'text-slate-400 group-hover:text-amber-400'
                      }`}
                    />
                    <span>{item.name}</span>
                  </div>
                  {item.badge && (
                    <span
                      className={`text-[10px] font-bold px-1.5 py-0.5 rounded uppercase tracking-wider ${
                        isActive
                          ? 'bg-slate-950 text-amber-400'
                          : 'bg-red-500/20 text-red-400 border border-red-500/30 animate-pulse'
                      }`}
                    >
                      {item.badge}
                    </span>
                  )}
                </a>
              );
            })}
        </nav>

        {/* Footer profile & logout */}
        {user && (
          <div className="p-4 bg-slate-950/40 border-t border-slate-800/80">
            <div className="flex items-center justify-between gap-3 mb-3">
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-white">{user.full_name}</p>
                <div className="flex items-center gap-1.5 mt-0.5">
                  <span className="inline-block h-2 w-2 rounded-full bg-emerald-400 animate-pulse" />
                  <span className="text-xs font-medium uppercase tracking-wider text-slate-400">
                    {user.system_role}
                  </span>
                </div>
              </div>
            </div>
            <button
              onClick={() => logout()}
              className="flex w-full items-center justify-center gap-2 rounded-lg bg-slate-800/80 px-3 py-2 text-xs font-semibold text-slate-300 hover:bg-red-500/10 hover:text-red-400 border border-slate-700/60 transition-colors"
            >
              <LogOut className="h-4 w-4" />
              Sign Out
            </button>
          </div>
        )}
      </aside>
    </>
  );
}
