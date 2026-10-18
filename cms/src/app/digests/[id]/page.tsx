'use client';

import React, { useState, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import apiClient, { getStoredTokens } from '@/lib/api/client';
import { useAuth } from '@/lib/auth/context';
import ProtectedRoute from '@/components/auth/ProtectedRoute';
import CMSLayout from '@/components/layout/CMSLayout';
import type { components } from '@/lib/api/schema';
import {
  Layers,
  ArrowLeft,
  Sparkles,
  Scissors,
  Send,
  BookOpen,
  Calendar,
  Scale,
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  FileText,
  Bookmark,
  Gavel,
  ChevronDown,
  ChevronUp,
  Tag,
  Hash,
} from 'lucide-react';

type DigestDetailView = components['schemas']['DigestDetailResponse'];
type Digest = components['schemas']['Digest'];
type DigestItem = components['schemas']['DigestItem'];

export default function DigestBuilderPage() {
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;
  const { user } = useAuth();

  const [detail, setDetail] = useState<DigestDetailView | null>(null);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [activeTab, setActiveTab] = useState<'nominal' | 'subject' | 'statute' | 'items' | 'preview'>('nominal');

  // Split parts modal
  const [showSplitModal, setShowSplitModal] = useState(false);
  const [splitPartCount, setSplitPartCount] = useState(2);

  const fetchDigest = async () => {
    setLoading(true);
    try {
      const tokens = getStoredTokens();
      const { data, error } = await apiClient.GET('/v1/cms/digests/{id}', {
        params: { path: { id } },
        headers: {
          Authorization: `Bearer ${tokens?.accessToken}`,
        },
      });

      if (error) {
        throw new Error('Failed to load digest details');
      }

      if (data) {
        setDetail(data);
      }
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Error loading digest' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (id) {
      fetchDigest();
    }
  }, [id]);

  const handleAutoAssemble = async () => {
    setActionLoading('assemble');
    setNotification(null);
    try {
      const tokens = getStoredTokens();
      const { data, error } = await apiClient.POST('/v1/cms/digests/{id}/auto-assemble', {
        params: { path: { id } },
        headers: {
          Authorization: `Bearer ${tokens?.accessToken}`,
        },
      });

      if (error) {
        throw new Error('Auto-assembly failed');
      }

      setNotification({
        type: 'success',
        message: `Successfully assembled ${data?.digest?.items?.length || 0} judgments and generated legal indexes!`,
      });
      if (data) {
        setDetail(data);
      }
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Failed to auto-assemble judgments' });
    } finally {
      setActionLoading(null);
    }
  };

  const handleSplitParts = async () => {
    setActionLoading('split');
    setNotification(null);
    try {
      const tokens = getStoredTokens();
      const { data, error } = await apiClient.POST('/v1/cms/digests/{id}/split-parts', {
        params: { path: { id } },
        body: { parts: splitPartCount },
        headers: {
          Authorization: `Bearer ${tokens?.accessToken}`,
        },
      });

      if (error) {
        throw new Error('Failed to split digest into parts');
      }

      setShowSplitModal(false);
      setNotification({
        type: 'success',
        message: `Successfully split digest into ${data?.count || 0} parts! Redirecting to digests list...`,
      });
      setTimeout(() => {
        router.push('/digests');
      }, 1500);
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Error splitting digest' });
    } finally {
      setActionLoading(null);
    }
  };

  const handlePublish = async () => {
    setActionLoading('publish');
    setNotification(null);
    try {
      const tokens = getStoredTokens();
      const { data, error } = await apiClient.POST('/v1/cms/digests/{id}/publish', {
        params: { path: { id } },
        headers: {
          Authorization: `Bearer ${tokens?.accessToken}`,
        },
      });

      if (error) {
        throw new Error('Publish failed');
      }

      setNotification({
        type: 'success',
        message: `Digest published successfully as a premium article (Slug: ${data?.article?.slug})!`,
      });
      fetchDigest();
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Error publishing digest' });
    } finally {
      setActionLoading(null);
    }
  };

  if (loading || !detail) {
    return (
      <ProtectedRoute allowedRoles={['editor', 'admin']}>
        <CMSLayout>
          <div className="flex flex-col items-center justify-center p-24 text-slate-500">
            <RefreshCw className="w-8 h-8 animate-spin text-indigo-600 mb-3" />
            <p className="text-sm">Loading digest builder...</p>
          </div>
        </CMSLayout>
      </ProtectedRoute>
    );
  }

  const d = detail.digest;
  const itemsCount = d.items?.length || 0;

  return (
    <ProtectedRoute allowedRoles={['editor', 'admin']}>
      <CMSLayout>
        <div className="space-y-6">
          {/* Top Bar Navigation */}
          <div className="flex flex-col lg:flex-row justify-between items-start lg:items-center gap-4 border-b border-slate-200 dark:border-slate-800 pb-5">
            <div className="flex items-center gap-3">
              <Link
                href="/digests"
                className="p-2 rounded-lg text-slate-500 hover:text-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
              >
                <ArrowLeft className="w-5 h-5" />
              </Link>
              <div>
                <div className="flex items-center gap-2">
                  <h1 className="text-xl font-bold text-slate-900 dark:text-white">{d.title}</h1>
                  <span
                    className={`text-xs px-2.5 py-0.5 rounded-full font-semibold uppercase tracking-wider ${
                      d.status === 'published'
                        ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-400'
                        : d.status === 'archived'
                        ? 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'
                        : 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-400'
                    }`}
                  >
                    {d.status}
                  </span>
                  <span className="text-xs px-2 py-0.5 rounded bg-indigo-50 text-indigo-700 dark:bg-indigo-950/50 dark:text-indigo-400 font-mono">
                    {d.court_code}
                  </span>
                </div>
                <div className="flex items-center gap-3 text-xs text-slate-500 mt-1">
                  <span>Period: {new Date(d.period_start).toLocaleDateString()} to {new Date(d.period_end).toLocaleDateString()}</span>
                  <span>•</span>
                  <span>Citations: {d.citation_range_start && d.citation_range_end ? `${d.citation_range_start} to ${d.citation_range_end}` : 'Auto-detected'}</span>
                  <span>•</span>
                  <span>{itemsCount} judgments compiled</span>
                </div>
              </div>
            </div>

            {/* Builder Action Buttons */}
            <div className="flex flex-wrap items-center gap-2.5 w-full lg:w-auto">
              <button
                onClick={handleAutoAssemble}
                disabled={actionLoading !== null}
                className="inline-flex items-center px-3.5 py-2 text-xs font-semibold rounded-lg bg-indigo-50 text-indigo-700 hover:bg-indigo-100 dark:bg-indigo-950/50 dark:text-indigo-300 dark:border dark:border-indigo-800 transition-colors disabled:opacity-50"
              >
                <Sparkles className={`w-3.5 h-3.5 mr-1.5 ${actionLoading === 'assemble' ? 'animate-spin' : ''}`} />
                {itemsCount > 0 ? 'Re-Assemble from Citations' : '⚡ Auto-Assemble Judgments'}
              </button>

              {itemsCount > 3 && (
                <button
                  onClick={() => setShowSplitModal(true)}
                  disabled={actionLoading !== null || d.status === 'published'}
                  className="inline-flex items-center px-3.5 py-2 text-xs font-semibold rounded-lg bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 transition-colors disabled:opacity-50"
                >
                  <Scissors className="w-3.5 h-3.5 mr-1.5" />
                  Split into Parts
                </button>
              )}

              <button
                onClick={handlePublish}
                disabled={actionLoading !== null || itemsCount === 0 || d.status === 'published'}
                className="inline-flex items-center px-4 py-2 text-xs font-semibold rounded-lg text-white bg-indigo-600 hover:bg-indigo-700 shadow-sm transition-colors disabled:opacity-50"
              >
                <Send className={`w-3.5 h-3.5 mr-1.5 ${actionLoading === 'publish' ? 'animate-spin' : ''}`} />
                {d.status === 'published' ? 'Published' : '🚀 Publish Digest Article'}
              </button>
            </div>
          </div>

          {/* Notification */}
          {notification && (
            <div
              className={`p-4 rounded-lg flex items-center justify-between text-sm ${
                notification.type === 'success'
                  ? 'bg-emerald-50 text-emerald-800 border border-emerald-200 dark:bg-emerald-950/30 dark:text-emerald-300 dark:border-emerald-800'
                  : 'bg-rose-50 text-rose-800 border border-rose-200 dark:bg-rose-950/30 dark:text-rose-300 dark:border-rose-800'
              }`}
            >
              <div className="flex items-center gap-2">
                {notification.type === 'success' ? (
                  <CheckCircle2 className="w-4 h-4 text-emerald-600 dark:text-emerald-400" />
                ) : (
                  <AlertCircle className="w-4 h-4 text-rose-600 dark:text-rose-400" />
                )}
                <span>{notification.message}</span>
              </div>
              <button onClick={() => setNotification(null)} className="text-xs font-semibold hover:underline">
                Dismiss
              </button>
            </div>
          )}

          {/* Builder Tabs */}
          <div className="flex border-b border-slate-200 dark:border-slate-800 overflow-x-auto">
            <button
              onClick={() => setActiveTab('nominal')}
              className={`px-4 py-2.5 text-xs font-bold border-b-2 flex items-center gap-1.5 whitespace-nowrap transition-colors ${
                activeTab === 'nominal'
                  ? 'border-indigo-600 text-indigo-600 dark:text-indigo-400 dark:border-indigo-400'
                  : 'border-transparent text-slate-500 hover:text-slate-900 dark:hover:text-white'
              }`}
            >
              <BookOpen className="w-3.5 h-3.5" />
              I. Nominal Index ({detail.nominal_index?.length || 0})
            </button>
            <button
              onClick={() => setActiveTab('subject')}
              className={`px-4 py-2.5 text-xs font-bold border-b-2 flex items-center gap-1.5 whitespace-nowrap transition-colors ${
                activeTab === 'subject'
                  ? 'border-indigo-600 text-indigo-600 dark:text-indigo-400 dark:border-indigo-400'
                  : 'border-transparent text-slate-500 hover:text-slate-900 dark:hover:text-white'
              }`}
            >
              <Tag className="w-3.5 h-3.5" />
              II. Subject Index ({detail.subject_index?.length || 0})
            </button>
            <button
              onClick={() => setActiveTab('statute')}
              className={`px-4 py-2.5 text-xs font-bold border-b-2 flex items-center gap-1.5 whitespace-nowrap transition-colors ${
                activeTab === 'statute'
                  ? 'border-indigo-600 text-indigo-600 dark:text-indigo-400 dark:border-indigo-400'
                  : 'border-transparent text-slate-500 hover:text-slate-900 dark:hover:text-white'
              }`}
            >
              <Scale className="w-3.5 h-3.5" />
              III. Statute Index ({detail.statute_index?.length || 0})
            </button>
            <button
              onClick={() => setActiveTab('items')}
              className={`px-4 py-2.5 text-xs font-bold border-b-2 flex items-center gap-1.5 whitespace-nowrap transition-colors ${
                activeTab === 'items'
                  ? 'border-indigo-600 text-indigo-600 dark:text-indigo-400 dark:border-indigo-400'
                  : 'border-transparent text-slate-500 hover:text-slate-900 dark:hover:text-white'
              }`}
            >
              <Gavel className="w-3.5 h-3.5" />
              IV. Judgments & Summaries ({itemsCount})
            </button>
            <button
              onClick={() => setActiveTab('preview')}
              className={`px-4 py-2.5 text-xs font-bold border-b-2 flex items-center gap-1.5 whitespace-nowrap transition-colors ${
                activeTab === 'preview'
                  ? 'border-indigo-600 text-indigo-600 dark:text-indigo-400 dark:border-indigo-400'
                  : 'border-transparent text-slate-500 hover:text-slate-900 dark:hover:text-white'
              }`}
            >
              <FileText className="w-3.5 h-3.5" />
              Full Article Preview
            </button>
          </div>

          {/* Tab 1: Nominal Index */}
          {activeTab === 'nominal' && (
            <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm">
              <div className="p-4 bg-slate-50 dark:bg-slate-800/40 border-b border-slate-200 dark:border-slate-800 flex justify-between items-center">
                <div>
                  <h3 className="text-sm font-bold text-slate-800 dark:text-slate-200">
                    Nominal Index (Alphabetical by Case Title)
                  </h3>
                  <p className="text-xs text-slate-500 mt-0.5">
                    Standard legal directory listing all appellant and respondent names in strict alphabetical order.
                  </p>
                </div>
              </div>
              {detail.nominal_index?.length === 0 ? (
                <div className="p-8 text-center text-slate-400 text-xs">
                  No judgments assembled yet. Click &quot;Auto-Assemble Judgments&quot; above.
                </div>
              ) : (
                <div className="divide-y divide-slate-100 dark:divide-slate-800">
                  {detail.nominal_index.map((entry, idx) => (
                    <div key={idx} className="p-4 flex items-center justify-between hover:bg-slate-50/50 dark:hover:bg-slate-800/30">
                      <div>
                        <span className="font-semibold text-slate-900 dark:text-white text-sm">{entry.case_title}</span>
                        <div className="text-xs text-slate-400 mt-0.5">Decided: {entry.date}</div>
                      </div>
                      <span className="font-mono text-xs px-2.5 py-1 bg-slate-100 dark:bg-slate-800 rounded font-bold text-slate-800 dark:text-slate-200">
                        {entry.citation}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Tab 2: Subject Index */}
          {activeTab === 'subject' && (
            <div className="space-y-4">
              <div className="bg-slate-50 dark:bg-slate-800/40 p-4 rounded-xl border border-slate-200 dark:border-slate-800">
                <h3 className="text-sm font-bold text-slate-800 dark:text-slate-200">
                  Subject Index (Thematic Topic Breakdown)
                </h3>
                <p className="text-xs text-slate-500 mt-0.5">
                  Categorized rulings grouped under constitutional, corporate, criminal, and statutory subjects.
                </p>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {detail.subject_index?.map((sub, idx) => (
                  <div key={idx} className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow-sm">
                    <div className="flex justify-between items-center pb-2 border-b border-slate-100 dark:border-slate-800 mb-3">
                      <span className="font-bold text-xs uppercase tracking-wider text-indigo-600 dark:text-indigo-400">
                        {sub.subject}
                      </span>
                      <span className="text-xs px-2 py-0.5 bg-slate-100 dark:bg-slate-800 rounded text-slate-600 dark:text-slate-300 font-semibold">
                        {sub.judgments.length} ruling(s)
                      </span>
                    </div>
                    <ul className="space-y-2 text-xs">
                      {sub.judgments.map((j, jIdx) => (
                        <li key={jIdx} className="flex justify-between items-center text-slate-700 dark:text-slate-300">
                          <span className="truncate pr-2 font-medium">{j.case_title}</span>
                          <span className="font-mono text-slate-400 whitespace-nowrap">{j.citation}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Tab 3: Statute Index */}
          {activeTab === 'statute' && (
            <div className="space-y-4">
              <div className="bg-slate-50 dark:bg-slate-800/40 p-4 rounded-xl border border-slate-200 dark:border-slate-800">
                <h3 className="text-sm font-bold text-slate-800 dark:text-slate-200">
                  Statute Index (Acts & Sections Cited)
                </h3>
                <p className="text-xs text-slate-500 mt-0.5">
                  Detailed statutory table linking rulings to specific sections of civil, criminal, and special statutes.
                </p>
              </div>
              <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm">
                <div className="divide-y divide-slate-100 dark:divide-slate-800">
                  {detail.statute_index?.map((st, idx) => (
                    <div key={idx} className="p-4 hover:bg-slate-50/50 dark:hover:bg-slate-800/30">
                      <div className="flex items-center gap-2 font-bold text-sm text-slate-900 dark:text-white">
                        <span>{st.statute_name}</span>
                        <span className="text-xs px-2 py-0.5 rounded bg-indigo-50 text-indigo-700 dark:bg-indigo-950/60 dark:text-indigo-300 font-mono">
                          {st.section_cited}
                        </span>
                      </div>
                      <div className="mt-2 space-y-1">
                        {st.judgments.map((j, jIdx) => (
                          <div key={jIdx} className="text-xs flex items-center justify-between text-slate-600 dark:text-slate-400 pl-3 border-l-2 border-indigo-200 dark:border-indigo-800">
                            <span>{j.case_title}</span>
                            <span className="font-mono text-slate-500">{j.citation}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}

          {/* Tab 4: Judgments & Blurbs */}
          {activeTab === 'items' && (
            <div className="space-y-4">
              {d.items?.map((it, idx) => (
                <div key={it.id} className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-5 shadow-sm space-y-3">
                  <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2">
                    <div className="flex items-center gap-2">
                      <span className="w-6 h-6 rounded-full bg-slate-100 dark:bg-slate-800 text-xs font-bold flex items-center justify-center text-slate-700 dark:text-slate-300">
                        {idx + 1}
                      </span>
                      <h4 className="font-bold text-sm text-slate-900 dark:text-white">{it.case_title}</h4>
                    </div>
                    <span className="font-mono text-xs font-bold text-indigo-600 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-950/50 px-2.5 py-1 rounded">
                      {it.citation}
                    </span>
                  </div>

                  {it.bench && it.bench.length > 0 && (
                    <div className="text-xs text-slate-500 font-medium">
                      Bench: {it.bench.join(', ')} • Date: {it.judgment_date}
                    </div>
                  )}

                  {it.headnote && (
                    <div className="text-xs bg-slate-50 dark:bg-slate-800/40 p-3 rounded-lg border border-slate-100 dark:border-slate-800">
                      <span className="font-semibold text-slate-700 dark:text-slate-300">Headnote: </span>
                      <span className="text-slate-600 dark:text-slate-400">{it.headnote}</span>
                    </div>
                  )}

                  {it.blurb && (
                    <div className="text-xs text-slate-700 dark:text-slate-300">
                      <span className="font-semibold">Held: </span>
                      {it.blurb}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Tab 5: Article Preview */}
          {activeTab === 'preview' && (
            <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-8 max-w-4xl mx-auto shadow-sm space-y-6">
              <div className="border-b border-slate-200 dark:border-slate-800 pb-4">
                <span className="text-xs font-bold uppercase tracking-wider text-indigo-600 dark:text-indigo-400">
                  {d.court} • Official Digest
                </span>
                <h2 className="text-2xl font-bold text-slate-900 dark:text-white mt-1">{d.title}</h2>
                <div className="text-xs text-slate-400 mt-1">
                  Access: <span className="font-semibold capitalize text-amber-500">{d.access_level}</span> • Period: {d.period}
                </div>
              </div>

              {d.intro && (
                <div className="text-sm text-slate-700 dark:text-slate-300 leading-relaxed italic border-l-4 border-indigo-500 pl-4">
                  {d.intro}
                </div>
              )}

              {/* Index TOC Preview */}
              <div className="space-y-4 bg-slate-50 dark:bg-slate-800/30 p-5 rounded-xl border border-slate-200 dark:border-slate-800">
                <h4 className="font-bold text-xs uppercase tracking-wider text-slate-700 dark:text-slate-300">
                  Table of Contents & Indexes
                </h4>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 text-xs">
                  <div className="p-3 bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700">
                    <span className="font-bold text-slate-900 dark:text-white block mb-1">Nominal Index</span>
                    <span className="text-slate-500">{detail.nominal_index?.length || 0} cases organized A to Z</span>
                  </div>
                  <div className="p-3 bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700">
                    <span className="font-bold text-slate-900 dark:text-white block mb-1">Subject Index</span>
                    <span className="text-slate-500">{detail.subject_index?.length || 0} legal topics cataloged</span>
                  </div>
                  <div className="p-3 bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700">
                    <span className="font-bold text-slate-900 dark:text-white block mb-1">Statute Index</span>
                    <span className="text-slate-500">{detail.statute_index?.length || 0} statutory sections cited</span>
                  </div>
                </div>
              </div>

              <div className="pt-4 border-t border-slate-200 dark:border-slate-800 text-xs text-slate-500 text-center">
                This compilation will be published as an official long-form digest article behind the subscriber paywall.
              </div>
            </div>
          )}

          {/* Split Parts Modal */}
          {showSplitModal && (
            <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
              <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 p-6 max-w-md w-full shadow-xl space-y-4">
                <h3 className="text-lg font-bold text-slate-900 dark:text-white flex items-center gap-2">
                  <Scissors className="w-5 h-5 text-indigo-600" />
                  Split Digest into Parts
                </h3>
                <p className="text-xs text-slate-500">
                  Large legal digests with dozens of rulings can be split into sequential parts (e.g. Part 1, Part 2) with continuous citation ranges.
                </p>

                <div>
                  <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-1.5">
                    Number of Sequential Parts
                  </label>
                  <input
                    type="number"
                    min={2}
                    max={10}
                    value={splitPartCount}
                    onChange={(e) => setSplitPartCount(parseInt(e.target.value, 10) || 2)}
                    className="w-full text-sm px-3.5 py-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white"
                  />
                  <span className="text-xs text-slate-400 mt-1 block">
                    Each part will contain ~{Math.ceil(itemsCount / splitPartCount)} rulings.
                  </span>
                </div>

                <div className="flex justify-end gap-3 pt-3">
                  <button
                    onClick={() => setShowSplitModal(false)}
                    className="px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleSplitParts}
                    disabled={actionLoading !== null}
                    className="px-4 py-2 text-xs font-semibold text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg shadow-sm transition-colors"
                  >
                    Confirm & Split
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      </CMSLayout>
    </ProtectedRoute>
  );
}
