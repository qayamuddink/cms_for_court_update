import React from 'react';
import './globals.css';
import { AuthProvider } from '@/lib/auth/context';

export const metadata = {
  title: 'LiveLaw CMS - Editorial & Court Desk',
  description: 'Private CMS for legal reporters and editors',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="bg-slate-50 text-slate-900 font-sans antialiased">
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
