'use client';

import React, { useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import apiClient, { getStoredTokens } from '@/lib/api/client';
import ProtectedRoute from '@/components/auth/ProtectedRoute';
import CMSLayout from '@/components/layout/CMSLayout';
import {
  Layers,
  ArrowLeft,
  Sparkles,
  AlertCircle,
  Calendar,
  Scale,
  Shield,
} from 'lucide-react';

const COMMON_COURTS = [
  { code: 'SC', name: 'Supreme Court of India' },
  { code: 'DEL', name: 'Delhi High Court' },
  { code: 'BOM', name: 'Bombay High Court' },
  { code: 'KER', name: 'Kerala High Court' },
  { code: 'ALL', name: 'Allahabad High Court' },
  { code: 'CAL', name: 'Calcutta High Court' },
  { code: 'MAD', name: 'Madras High Court' },
  { code: 'KAR', name: 'Karnataka High Court' },
];

export default function NewDigestPage() {
  const router = useRouter();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [court, setCourt] = useState('Supreme Court of India');
  const [courtCode, setCourtCode] = useState('SC');
  const [digestType, setDigestType] = useState<'digest' | 'round_up'>('digest');
  const [period, setPeriod] = useState<'weekly' | 'monthly' | 'quarterly' | 'annual' | 'thematic'>('monthly');

  // Default to previous month
  const now = new Date();
  const firstDayPrevMonth = new Date(now.getFullYear(), now.getMonth() - 1, 1).toISOString().split('T')[0];
  const lastDayPrevMonth = new Date(now.getFullYear(), now.getMonth(), 0).toISOString().split('T')[0];

  const [periodStart, setPeriodStart] = useState(firstDayPrevMonth);
  const [periodEnd, setPeriodEnd] = useState(lastDayPrevMonth);
  const [title, setTitle] = useState(`Supreme Court Monthly Digest: ${new Date(firstDayPrevMonth).toLocaleString('default', { month: 'long', year: 'numeric' })}`);
  const [citationRangeStart, setCitationRangeStart] = useState('');
  const [citationRangeEnd, setCitationRangeEnd] = useState('');
  const [citationPrefix, setCitationPrefix] = useState(`2026 LiveLaw (SC)`);
  const [intro, setIntro] = useState('A comprehensive legal digest of landmark rulings and principles laid down by the court.');
  const [accessLevel, setAccessLevel] = useState<'premium' | 'free'>('premium');

  const handleCourtChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const selected = COMMON_COURTS.find((c) => c.code === e.target.value);
    if (selected) {
      setCourt(selected.name);
      setCourtCode(selected.code);
      setCitationPrefix(`2026 LiveLaw (${selected.code})`);
      setTitle(`${selected.name} ${period.charAt(0).toUpperCase() + period.slice(1)} Digest: ${new Date(periodStart).toLocaleString('default', { month: 'long', year: 'numeric' })}`);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);

    try {
      const tokens = getStoredTokens();
      const payload: any = {
        title: title.trim(),
        court,
        court_code: courtCode,
        digest_type: digestType,
        period,
        period_start: periodStart,
        period_end: periodEnd,
        citation_prefix: citationPrefix,
        intro: intro.trim(),
        access_level: accessLevel,
      };

      if (citationRangeStart) payload.citation_range_start = parseInt(citationRangeStart, 10);
      if (citationRangeEnd) payload.citation_range_end = parseInt(citationRangeEnd, 10);

      const { data, error: apiError } = await apiClient.POST('/v1/cms/digests', {
        body: payload,
        headers: {
          Authorization: `Bearer ${tokens?.accessToken}`,
        },
      });

      if (apiError) {
        throw new Error((apiError as any).message || 'Failed to create digest');
      }

      if (data?.id) {
        router.push(`/digests/${data.id}`);
      }
    } catch (err: any) {
      setError(err.message || 'Error creating digest draft');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <ProtectedRoute allowedRoles={['editor', 'admin']}>
      <CMSLayout>
        <div className="max-w-3xl mx-auto space-y-6">
          <div className="flex items-center gap-3">
            <Link
              href="/digests"
              className="p-2 rounded-lg text-slate-500 hover:text-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
            >
              <ArrowLeft className="w-5 h-5" />
            </Link>
            <div>
              <h1 className="text-xl font-bold text-slate-900 dark:text-white">Create New Digest Draft</h1>
              <p className="text-xs text-slate-500 mt-0.5">
                Set up court, period, and citation criteria. You can auto-assemble judgments on the next step.
              </p>
            </div>
          </div>

          {error && (
            <div className="p-4 rounded-lg bg-rose-50 border border-rose-200 text-rose-800 dark:bg-rose-950/40 dark:border-rose-800 dark:text-rose-300 flex items-center gap-2 text-sm">
              <AlertCircle className="w-4 h-4 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <form onSubmit={handleSubmit} className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-sm space-y-5">
            {/* Title */}
            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                Digest Title
              </label>
              <input
                type="text"
                required
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                className="w-full text-sm px-3.5 py-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white focus:ring-2 focus:ring-indigo-500"
                placeholder="e.g. Supreme Court Monthly Digest: January 2026"
              />
            </div>

            {/* Court & Type */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                  Court / Jurisdiction
                </label>
                <select
                  value={courtCode}
                  onChange={handleCourtChange}
                  className="w-full text-sm px-3.5 py-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white"
                >
                  {COMMON_COURTS.map((c) => (
                    <option key={c.code} value={c.code}>
                      {c.name} ({c.code})
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                  Compilation Type
                </label>
                <select
                  value={digestType}
                  onChange={(e) => setDigestType(e.target.value as any)}
                  className="w-full text-sm px-3.5 py-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white"
                >
                  <option value="digest">Full Comprehensive Digest (with Nominal, Subject & Statute Index)</option>
                  <option value="round_up">High Court Round-up (Highlights & Blurbs)</option>
                </select>
              </div>
            </div>

            {/* Period & Dates */}
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                  Period Cadence
                </label>
                <select
                  value={period}
                  onChange={(e) => setPeriod(e.target.value as any)}
                  className="w-full text-sm px-3.5 py-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white"
                >
                  <option value="weekly">Weekly</option>
                  <option value="monthly">Monthly</option>
                  <option value="quarterly">Quarterly</option>
                  <option value="annual">Annual</option>
                  <option value="thematic">Thematic</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                  Period Start Date
                </label>
                <input
                  type="date"
                  required
                  value={periodStart}
                  onChange={(e) => setPeriodStart(e.target.value)}
                  className="w-full text-sm px-3.5 py-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                  Period End Date
                </label>
                <input
                  type="date"
                  required
                  value={periodEnd}
                  onChange={(e) => setPeriodEnd(e.target.value)}
                  className="w-full text-sm px-3.5 py-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white"
                />
              </div>
            </div>

            {/* Citation Range Filtering */}
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                  Citation Prefix
                </label>
                <input
                  type="text"
                  value={citationPrefix}
                  onChange={(e) => setCitationPrefix(e.target.value)}
                  className="w-full text-sm px-3.5 py-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white"
                  placeholder="e.g. 2026 LiveLaw (SC)"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                  Citation Start (Optional)
                </label>
                <input
                  type="number"
                  value={citationRangeStart}
                  onChange={(e) => setCitationRangeStart(e.target.value)}
                  className="w-full text-sm px-3.5 py-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white"
                  placeholder="e.g. 1"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                  Citation End (Optional)
                </label>
                <input
                  type="number"
                  value={citationRangeEnd}
                  onChange={(e) => setCitationRangeEnd(e.target.value)}
                  className="w-full text-sm px-3.5 py-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white"
                  placeholder="e.g. 50"
                />
              </div>
            </div>

            {/* Access Level */}
            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                Access Level & Paywall
              </label>
              <div className="flex gap-4">
                <label className="flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300 cursor-pointer">
                  <input
                    type="radio"
                    name="accessLevel"
                    value="premium"
                    checked={accessLevel === 'premium'}
                    onChange={() => setAccessLevel('premium')}
                    className="text-indigo-600 focus:ring-indigo-500"
                  />
                  <span>🔒 Premium Only (Subscribers)</span>
                </label>
                <label className="flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300 cursor-pointer">
                  <input
                    type="radio"
                    name="accessLevel"
                    value="free"
                    checked={accessLevel === 'free'}
                    onChange={() => setAccessLevel('free')}
                    className="text-indigo-600 focus:ring-indigo-500"
                  />
                  <span>Free (Public Access)</span>
                </label>
              </div>
            </div>

            {/* Intro text */}
            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                Executive Intro / Summary
              </label>
              <textarea
                rows={3}
                value={intro}
                onChange={(e) => setIntro(e.target.value)}
                className="w-full text-sm px-3.5 py-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white"
                placeholder="Opening remarks explaining key highlights..."
              />
            </div>

            {/* Submit */}
            <div className="flex justify-end gap-3 pt-4 border-t border-slate-200 dark:border-slate-800">
              <Link
                href="/digests"
                className="px-4 py-2 text-sm font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg transition-colors"
              >
                Cancel
              </Link>
              <button
                type="submit"
                disabled={submitting}
                className="inline-flex items-center px-5 py-2 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg shadow-sm transition-colors disabled:opacity-50"
              >
                {submitting ? 'Creating...' : 'Create & Open Builder →'}
              </button>
            </div>
          </form>
        </div>
      </CMSLayout>
    </ProtectedRoute>
  );
}
