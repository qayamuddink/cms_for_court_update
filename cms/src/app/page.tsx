import React from 'react';

export default function HomePage() {
  return (
    <main style={{ padding: '3rem', maxWidth: '800px', margin: '0 auto', textAlign: 'center' }}>
      <h1 style={{ fontSize: '2.5rem', marginBottom: '1rem', color: '#1e293b' }}>
        LiveLaw Legal Journalism CMS
      </h1>
      <p style={{ fontSize: '1.2rem', color: '#64748b', lineHeight: '1.6' }}>
        Phase 1 placeholder active. Editorial suite and Live Court Desk will be implemented in Phase 7 &amp; 8.
      </p>
      <div style={{ marginTop: '2rem', display: 'inline-block', padding: '0.75rem 1.5rem', background: '#e2e8f0', borderRadius: '0.5rem', fontWeight: 600 }}>
        Status: CMS Container Running
      </div>
    </main>
  );
}
