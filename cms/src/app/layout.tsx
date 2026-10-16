import React from 'react';

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
      <body style={{ margin: 0, fontFamily: 'system-ui, -apple-system, sans-serif' }}>
        {children}
      </body>
    </html>
  );
}
