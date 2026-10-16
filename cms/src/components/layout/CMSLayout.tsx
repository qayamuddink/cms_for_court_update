'use client';

import React, { useState } from 'react';
import Sidebar from './Sidebar';
import Header from './Header';

interface CMSLayoutProps {
  children: React.ReactNode;
  title?: string;
  badge?: string;
}

export default function CMSLayout({ children, title, badge }: CMSLayoutProps) {
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div className="min-h-screen bg-slate-50 flex">
      {/* Sidebar (fixed desktop, drawer mobile) */}
      <Sidebar isOpen={sidebarOpen} onClose={() => setSidebarOpen(false)} />

      {/* Main content viewport */}
      <div className="flex-1 flex flex-col min-w-0 lg:pl-72">
        <Header
          onOpenSidebar={() => setSidebarOpen(true)}
          title={title}
          badge={badge}
        />
        <main className="flex-1 p-4 sm:p-6 lg:p-8 max-w-7xl w-full mx-auto">
          {children}
        </main>
      </div>
    </div>
  );
}
