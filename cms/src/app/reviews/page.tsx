'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';

export default function ReviewsRedirectPage() {
  const router = useRouter();

  useEffect(() => {
    router.replace('/articles?status=in_review');
  }, [router]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-900 text-white text-xs font-semibold">
      Loading editorial review queue...
    </div>
  );
}
