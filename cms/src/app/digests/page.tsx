'use client';

import React, { useState, useEffect } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import apiClient, { getStoredTokens } from '@/lib/api/client';
import { useAuth } from '@/lib/auth/context';
import ProtectedRoute from '@/components/auth/ProtectedRoute';
import CMSLayout from '@/components/layout/CMSLayout';
import type { components } from '@/lib/api/schema';
import {
  Layers,
  Plus,
  RefreshCw,
  Sparkles,
  Calendar,
  Scale,
  Clock,
  CheckCircle2,
  AlertCircle,
  ChevronRight,
  ExternalLink,
  BookOpen,
  Filter,
} from 'lucide-react';

type Digest = components['schemas']['Digest'];

export default function DigestsPage() {
  const router = useRouter();
  const { user } = useAuth();
  const [digests, setDigests] = useState<Digest[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [generatingMonthly, setGeneratingMonthly] = useState(false);
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  // Filters
  const [courtFilter, setCourtFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [periodFilter, setPeriodFilter] = useState('');

  const fetchDigests = async () => {
    setLoading(true);
    try {
      const tokens = getStoredTokens();
      const params: Record<string, string> = {};
      if (courtFilter) params.court = courtFilter;
      if (statusFilter) params.status = statusFilter;
      if (periodFilter) params.period = periodFilter;

      const { data, error } = await apiClient.GET('/v1/cms/digests', {
        params: { query: params },
        headers: {
          Authorization: `Bearer ${tokens?.accessToken}`,
        },
      });

      if (error) {
        throw new Error('Failed to load digests');
      }

      if (data) {
        setDigests(data.digests || []);
        setTotal(data.total || 0);
      }
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Error loading digests' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDigests();
  }, [courtFilter, statusFilter, periodFilter]);

  const handleGenerateMonthly = async () => {
    setGeneratingMonthly(true);
    setNotification(null);
    try {
      const tokens = getStoredTokens();
      const { data, error } = await apiClient.POST('/v1/cms/digests/generate-monthly', {
        headers: {
          Authorization: `Bearer ${tokens?.accessToken}`,
        },
      });

      if (error) {
        throw new Error('Failed to generate monthly digests');
      }

      setNotification({
        type: 'success',
        message: `Successfully generated ${data?.count || 0} monthly digest draft(s) for key courts!`,
      });
      fetchDigests();
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Error generating monthly digests' });
    } finally {
      setGeneratingMonthly(false);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'published':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-emerald-100 text-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-400">
            <CheckCircle2 className="w-3 h-3 mr-1" /> Published
          </span>
        );
      case 'archived':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-400">
            Archived (Split)
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-amber-100 text-amber-800 dark:bg-amber-950/50 dark:text-amber-400">
            <Clock className="w-3 h-3 mr-1" /> Draft
          </span>
        );
    }
  };

  return (
    <ProtectedRoute allowedRoles={['editor', 'admin']}>
      <CMSLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
            <div>
              <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-100 flex items-center gap-2">
                <Layers className="w-6 h-6 text-indigo-600 dark:text-indigo-400" />
                Digests & Round-ups
              </h1>
              <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">
                Weekly, monthly, quarterly and annual legal digests auto-assembled from official citations.
              </p>
            </div>
            <div className="flex items-center gap-3 w-full sm:w-auto">
              <button
                onClick={handleGenerateMonthly}
                disabled={generatingMonthly}
                className="inline-flex items-center justify-center px-4 py-2 text-sm font-medium text-indigo-700 bg-indigo-50 border border-indigo-200 rounded-lg hover:bg-indigo-100 dark:bg-indigo-950/40 dark:text-indigo-300 dark:border-indigo-800 transition-colors disabled:opacity-50"
              >
                <Sparkles className={`w-4 h-4 mr-2 ${generatingMonthly ? 'animate-spin' : ''}`} />
                {generatingMonthly ? 'Generating...' : 'Auto-Generate Monthly'}
              </button>
              <Link
                href="/digests/new"
                className="inline-flex items-center justify-center px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 shadow-sm transition-colors"
              >
                <Plus className="w-4 h-4 mr-1.5" />
                New Digest
              </Link>
            </div>
          </div>

          {/* Alert Notification */}
          {notification && (
            <div
              className={`p-4 rounded-lg flex items-center justify-between ${
                notification.type === 'success'
                  ? 'bg-emerald-50 text-emerald-800 border border-emerald-200 dark:bg-emerald-950/30 dark:text-emerald-300 dark:border-emerald-800'
                  : 'bg-rose-50 text-rose-800 border border-rose-200 dark:bg-rose-950/30 dark:text-rose-300 dark:border-rose-800'
              }`}
            >
              <div className="flex items-center gap-2 text-sm">
                {notification.type === 'success' ? (
                  <CheckCircle2 className="w-4 h-4 text-emerald-600 dark:text-emerald-400" />
                ) : (
                  <AlertCircle className="w-4 h-4 text-rose-600 dark:text-rose-400" />
                )}
                <span>{notification.message}</span>
              </div>
              <button
                onClick={() => setNotification(null)}
                className="text-xs font-semibold hover:underline"
              >
                Dismiss
              </button>
            </div>
          )}

          {/* Quick Metrics */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow-sm">
              <span className="text-xs font-medium text-slate-500 uppercase tracking-wider">Total Compilations</span>
              <p className="text-2xl font-bold text-slate-900 dark:text-white mt-1">{total}</p>
            </div>
            <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow-sm">
              <span className="text-xs font-medium text-slate-500 uppercase tracking-wider">Published</span>
              <p className="text-2xl font-bold text-emerald-600 dark:text-emerald-400 mt-1">
                {digests.filter((d) => d.status === 'published').length}
              </p>
            </div>
            <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow-sm">
              <span className="text-xs font-medium text-slate-500 uppercase tracking-wider">Drafts</span>
              <p className="text-2xl font-bold text-amber-600 dark:text-amber-400 mt-1">
                {digests.filter((d) => d.status === 'draft').length}
              </p>
            </div>
            <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow-sm">
              <span className="text-xs font-medium text-slate-500 uppercase tracking-wider">Round-ups</span>
              <p className="text-2xl font-bold text-indigo-600 dark:text-indigo-400 mt-1">
                {digests.filter((d) => d.digest_type === 'round_up').length}
              </p>
            </div>
          </div>

          {/* Filters Bar */}
          <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-4 flex flex-wrap gap-4 items-center justify-between">
            <div className="flex flex-wrap items-center gap-3">
              <div className="flex items-center gap-1.5 text-xs text-slate-500 font-medium">
                <Filter className="w-3.5 h-3.5" /> Filters:
              </div>
              <select
                value={courtFilter}
                onChange={(e) => setCourtFilter(e.target.value)}
                className="text-xs px-2.5 py-1.5 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-700 dark:text-slate-300"
              >
                <option value="">All Courts</option>
                <option value="SC">Supreme Court (SC)</option>
                <option value="DEL">Delhi High Court (DEL)</option>
                <option value="BOM">Bombay High Court (BOM)</option>
                <option value="KER">Kerala High Court (KER)</option>
              </select>

              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                className="text-xs px-2.5 py-1.5 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-700 dark:text-slate-300"
              >
                <option value="">All Statuses</option>
                <option value="draft">Drafts</option>
                <option value="published">Published</option>
                <option value="archived">Archived</option>
              </select>

              <select
                value={periodFilter}
                onChange={(e) => setPeriodFilter(e.target.value)}
                className="text-xs px-2.5 py-1.5 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-slate-700 dark:text-slate-300"
              >
                <option value="">All Periods</option>
                <option value="weekly">Weekly</option>
                <option value="monthly">Monthly</option>
                <option value="quarterly">Quarterly</option>
                <option value="annual">Annual</option>
                <option value="thematic">Thematic</option>
              </select>
            </div>

            <button
              onClick={fetchDigests}
              className="inline-flex items-center text-xs text-slate-500 hover:text-slate-900 dark:hover:text-white"
            >
              <RefreshCw className={`w-3.5 h-3.5 mr-1 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </button>
          </div>

          {/* Digests Table */}
          <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm">
            {loading ? (
              <div className="p-12 text-center text-slate-500">
                <RefreshCw className="w-6 h-6 animate-spin mx-auto mb-2 text-indigo-600" />
                Loading digests...
              </div>
            ) : digests.length === 0 ? (
              <div className="p-12 text-center text-slate-500">
                <Layers className="w-10 h-10 mx-auto text-slate-300 dark:text-slate-700 mb-3" />
                <h3 className="font-semibold text-slate-800 dark:text-slate-200">No digests found</h3>
                <p className="text-xs text-slate-500 mt-1">Create a new digest or trigger auto-generation above.</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead className="bg-slate-50 dark:bg-slate-800/50 text-xs text-slate-500 uppercase border-b border-slate-200 dark:border-slate-800">
                    <tr>
                      <th className="px-6 py-3">Digest / Round-up</th>
                      <th className="px-6 py-3">Court & Period</th>
                      <th className="px-6 py-3">Citation Range</th>
                      <th className="px-6 py-3">Part Info</th>
                      <th className="px-6 py-3">Access</th>
                      <th className="px-6 py-3">Status</th>
                      <th className="px-6 py-3 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                    {digests.map((d) => (
                      <tr key={d.id} className="hover:bg-slate-50/50 dark:hover:bg-slate-800/50 transition-colors">
                        <td className="px-6 py-4">
                          <Link
                            href={`/digests/${d.id}`}
                            className="font-semibold text-slate-900 dark:text-white hover:text-indigo-600 dark:hover:text-indigo-400"
                          >
                            {d.title}
                          </Link>
                          <div className="text-xs text-slate-400 font-mono mt-0.5">{d.slug}</div>
                        </td>
                        <td className="px-6 py-4">
                          <div className="font-medium text-slate-800 dark:text-slate-200">{d.court}</div>
                          <div className="text-xs text-slate-500 capitalize">{d.period} ({d.digest_type})</div>
                        </td>
                        <td className="px-6 py-4">
                          {d.citation_range_start && d.citation_range_end ? (
                            <span className="font-mono text-xs bg-slate-100 dark:bg-slate-800 px-2 py-1 rounded">
                              {d.citation_range_start} – {d.citation_range_end}
                            </span>
                          ) : (
                            <span className="text-xs text-slate-400">All available</span>
                          )}
                        </td>
                        <td className="px-6 py-4">
                          {d.is_part ? (
                            <span className="text-xs font-medium text-indigo-600 dark:text-indigo-400">
                              Part {d.part_number} of {d.total_parts}
                            </span>
                          ) : (
                            <span className="text-xs text-slate-400">Full</span>
                          )}
                        </td>
                        <td className="px-6 py-4">
                          <span
                            className={`text-xs uppercase font-bold tracking-wider px-2 py-0.5 rounded ${
                              d.access_level === 'premium'
                                ? 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-400'
                                : 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'
                            }`}
                          >
                            {d.access_level}
                          </span>
                        </td>
                        <td className="px-6 py-4">{getStatusBadge(d.status)}</td>
                        <td className="px-6 py-4 text-right">
                          <Link
                            href={`/digests/${d.id}`}
                            className="inline-flex items-center text-xs font-semibold text-indigo-600 dark:text-indigo-400 hover:text-indigo-800"
                          >
                            Open Builder
                            <ChevronRight className="w-3.5 h-3.5 ml-0.5" />
                          </Link>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      </CMSLayout>
    </ProtectedRoute>
  );
}
