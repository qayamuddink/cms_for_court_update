'use client';

import React, { useState, useEffect } from 'react';
import apiClient from '@/lib/api/client';
import { useAuth } from '@/lib/auth/context';
import ProtectedRoute from '@/components/auth/ProtectedRoute';
import CMSLayout from '@/components/layout/CMSLayout';
import PDFUploader, { UploadedPDFMeta } from '@/components/articles/PDFUploader';
import {
  BookOpen,
  Plus,
  Scale,
  FileText,
  Sparkles,
  ExternalLink,
  Loader2,
  CheckCircle2,
  AlertCircle,
  Paperclip,
  Download,
  Search,
} from 'lucide-react';

interface JudgmentItem {
  id: string;
  slug: string;
  title: string;
  summary: string;
  court_case_id?: string | null;
  court_name?: string | null;
  citation?: string | null;
  access_level: 'free' | 'premium';
  status: string;
  version: number;
  pdf_storage_key?: string | null;
  pdf_size_bytes?: number | null;
  pdf_checksum?: string | null;
  created_at: string;
  updated_at: string;
}

export default function JudgmentsPage() {
  const { user, hasRole } = useAuth();

  const [judgments, setJudgments] = useState<JudgmentItem[]>([]);
  const [liveCases, setLiveCases] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  // Form Fields for Create Judgment
  const [courtCaseId, setCourtCaseId] = useState('');
  const [title, setTitle] = useState('');
  const [summary, setSummary] = useState('');
  const [courtName, setCourtName] = useState('Supreme Court of India');
  const [citation, setCitation] = useState('');
  const [accessLevel, setAccessLevel] = useState<'free' | 'premium'>('free');
  const [pdfMeta, setPdfMeta] = useState<UploadedPDFMeta | null>(null);

  const [creating, setCreating] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [successBanner, setSuccessBanner] = useState<string | null>(null);

  const fetchJudgments = async () => {
    setLoading(true);
    try {
      const { data, response } = await apiClient.GET('/v1/cms/articles', {
        params: { query: { type: 'judgment_summary', limit: 50 } },
      });
      if (response.ok && data) {
        setJudgments(data.articles as JudgmentItem[]);
      }
    } catch {
      // Ignore background error
    } finally {
      setLoading(false);
    }
  };

  const fetchCases = async () => {
    try {
      const { data } = await apiClient.GET('/v1/cases/live');
      const items = (data as any)?.cases || (Array.isArray(data) ? data : []);
      setLiveCases(items);
      if (items.length > 0 && !courtCaseId) {
        setCourtCaseId(items[0].id);
        if (items[0].court_name) setCourtName(items[0].court_name);
      }
    } catch {
      // Ignore
    }
  };

  useEffect(() => {
    fetchJudgments();
    fetchCases();
  }, []);

  const handleCaseSelect = (caseId: string) => {
    setCourtCaseId(caseId);
    const match = liveCases.find((c) => c.id === caseId);
    if (match) {
      if (match.court_name) setCourtName(match.court_name);
      if (match.title && !title) setTitle(`Judgment: ${match.title}`);
    }
  };

  const handleCreateJudgment = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!courtCaseId) {
      setFormError('Please select a linked court case.');
      return;
    }
    if (!title.trim() || !summary.trim()) {
      setFormError('Title and Summary are required.');
      return;
    }

    setCreating(true);
    setFormError(null);

    try {
      const { data, error, response } = await apiClient.POST('/v1/cms/judgments', {
        body: {
          court_case_id: courtCaseId,
          title: title.trim(),
          summary: summary.trim(),
          access_level: accessLevel,
          court_name: courtName.trim(),
          citation: citation.trim() ? citation.trim() : undefined,
          pdf_storage_key: pdfMeta?.storage_key,
          pdf_size_bytes: pdfMeta?.size_bytes,
          pdf_checksum: pdfMeta?.checksum,
        },
      });

      if (!response.ok || !data) {
        throw new Error((error as any)?.message || 'Failed to create judgment summary');
      }

      setSuccessBanner(`Judgment summary created and published successfully: "${data.title}"`);
      setModalOpen(false);
      // Reset form
      setTitle('');
      setSummary('');
      setCitation('');
      setPdfMeta(null);
      fetchJudgments();
    } catch (err: any) {
      setFormError(err.message || 'Error creating judgment summary');
    } finally {
      setCreating(false);
    }
  };

  const filteredJudgments = judgments.filter((j) => {
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase();
    return (
      j.title.toLowerCase().includes(q) ||
      (j.citation && j.citation.toLowerCase().includes(q)) ||
      (j.court_name && j.court_name.toLowerCase().includes(q))
    );
  });

  return (
    <ProtectedRoute allowedRoles={['editor', 'admin']}>
      <CMSLayout title="Judgments & Official Orders Desk" badge={`${judgments.length} Judgments`}>
        {/* Success Alert */}
        {successBanner && (
          <div className="mb-6 flex items-center justify-between rounded-xl bg-emerald-50 border border-emerald-200 p-4 text-xs font-bold text-emerald-800">
            <span>{successBanner}</span>
            <button
              type="button"
              onClick={() => setSuccessBanner(null)}
              className="underline ml-4"
            >
              Dismiss
            </button>
          </div>
        )}

        {/* Top Controls: Search & New Judgment */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-4 mb-6">
          <div className="relative flex-1 max-w-md">
            <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400" />
            <input
              type="text"
              placeholder="Search by judgment title, citation, or court..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full rounded-xl border border-slate-200 bg-white pl-10 pr-4 py-2 text-xs text-slate-800 placeholder:text-slate-400 focus:border-amber-400 focus:outline-none focus:ring-1 focus:ring-amber-400 shadow-sm"
            />
          </div>

          <button
            type="button"
            onClick={() => {
              setFormError(null);
              setModalOpen(true);
            }}
            className="inline-flex items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 px-5 py-2.5 text-xs font-extrabold text-slate-950 shadow-md shadow-amber-500/20 hover:from-amber-400 hover:to-amber-500 transition"
          >
            <Plus className="h-4 w-4 stroke-[2.5]" />
            New Judgment Summary
          </button>
        </div>

        {/* Judgments Grid */}
        {loading ? (
          <div className="py-24 text-center text-slate-400">
            <Loader2 className="h-8 w-8 animate-spin mx-auto mb-3 text-amber-500" />
            <p className="text-xs font-semibold">Loading official judgments database...</p>
          </div>
        ) : filteredJudgments.length === 0 ? (
          <div className="rounded-2xl bg-white border border-slate-200 p-12 text-center shadow-sm">
            <BookOpen className="h-12 w-12 text-slate-300 mx-auto mb-3" />
            <p className="text-base font-bold text-slate-700">No judgments found</p>
            <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
              Click &quot;New Judgment Summary&quot; to publish an official judgment with attached PDF.
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
            {filteredJudgments.map((j) => (
              <div
                key={j.id}
                className="rounded-2xl bg-white border border-slate-200 p-5 shadow-sm hover:border-slate-300 transition space-y-3 flex flex-col justify-between"
              >
                <div>
                  <div className="flex items-center justify-between gap-2 flex-wrap mb-2">
                    <span className="text-[10px] uppercase tracking-wider font-extrabold px-2 py-0.5 rounded bg-blue-100 text-blue-900 border border-blue-200 flex items-center gap-1">
                      <Scale className="h-3 w-3" />
                      {j.court_name || 'Supreme Court of India'}
                    </span>
                    {j.citation && (
                      <span className="font-mono text-xs font-bold text-amber-900 bg-amber-50 px-2 py-0.5 rounded border border-amber-200">
                        {j.citation}
                      </span>
                    )}
                    {j.access_level === 'premium' ? (
                      <span className="text-[10px] uppercase tracking-wider font-extrabold bg-amber-100 text-amber-900 px-2 py-0.5 rounded border border-amber-300 flex items-center gap-1">
                        <Sparkles className="h-3 w-3 text-amber-700" />
                        Premium
                      </span>
                    ) : (
                      <span className="text-[10px] uppercase tracking-wider font-semibold text-slate-500">
                        Free
                      </span>
                    )}
                  </div>

                  <h3 className="text-base font-extrabold text-slate-900 leading-snug">
                    <a href={`/articles/${j.id}/edit`} className="hover:text-amber-600 transition">
                      {j.title}
                    </a>
                  </h3>

                  <p className="text-xs text-slate-600 line-clamp-3 leading-relaxed mt-2">
                    {j.summary}
                  </p>
                </div>

                <div className="pt-3 border-t border-slate-100 flex items-center justify-between flex-wrap gap-2 text-xs">
                  {j.pdf_storage_key ? (
                    <span className="inline-flex items-center gap-1.5 font-bold text-emerald-700 bg-emerald-50 px-2.5 py-1 rounded-lg border border-emerald-200">
                      <Paperclip className="h-3.5 w-3.5" />
                      Official PDF Attached
                    </span>
                  ) : (
                    <span className="text-[11px] text-slate-400">No PDF attached</span>
                  )}

                  <div className="flex items-center gap-2">
                    <a
                      href={`/articles/${j.id}/edit`}
                      className="rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-700 px-3 py-1 font-bold text-xs transition"
                    >
                      Edit
                    </a>
                    <a
                      href={`http://localhost:8080/v1/judgments/${j.id}`}
                      target="_blank"
                      rel="noreferrer"
                      className="rounded-lg border border-slate-200 hover:bg-slate-50 text-slate-600 px-2.5 py-1 text-xs font-semibold flex items-center gap-1 transition"
                      title="Inspect Public API Response"
                    >
                      <ExternalLink className="h-3.5 w-3.5" />
                      API
                    </a>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Modal: Create Judgment Summary */}
        {modalOpen && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 backdrop-blur-sm p-4 overflow-y-auto animate-in fade-in">
            <div className="w-full max-w-xl rounded-2xl bg-white p-6 shadow-2xl border border-slate-200 my-8">
              <div className="flex items-center justify-between pb-3 border-b border-slate-100 mb-4">
                <div className="flex items-center gap-2">
                  <Scale className="h-5 w-5 text-amber-500" />
                  <h3 className="text-base font-extrabold text-slate-900">
                    Publish Official Judgment Summary
                  </h3>
                </div>
                <button
                  type="button"
                  onClick={() => setModalOpen(false)}
                  className="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-800"
                >
                  ✕
                </button>
              </div>

              {formError && (
                <div className="mb-4 flex items-center gap-2 rounded-xl bg-red-50 border border-red-200 p-3 text-xs font-semibold text-red-700">
                  <AlertCircle className="h-4 w-4 shrink-0 text-red-600" />
                  <span>{formError}</span>
                </div>
              )}

              <form onSubmit={handleCreateJudgment} className="space-y-4">
                <div>
                  <label className="block text-xs font-bold uppercase tracking-wider text-slate-700 mb-1">
                    Select Court Case *
                  </label>
                  <select
                    required
                    value={courtCaseId}
                    onChange={(e) => handleCaseSelect(e.target.value)}
                    className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2.5 text-xs font-bold text-slate-800 focus:border-amber-400 focus:outline-none"
                  >
                    <option value="">-- Choose court case --</option>
                    {liveCases.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.case_number ? `${c.case_number} - ` : ''}{c.title}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Court Name *
                    </label>
                    <input
                      type="text"
                      required
                      value={courtName}
                      onChange={(e) => setCourtName(e.target.value)}
                      placeholder="Supreme Court of India"
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Official Citation
                    </label>
                    <input
                      type="text"
                      value={citation}
                      onChange={(e) => setCitation(e.target.value)}
                      placeholder="e.g. 2026 INSC 44"
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>
                </div>

                <div>
                  <label className="block text-xs font-bold uppercase tracking-wider text-slate-700 mb-1">
                    Judgment Title *
                  </label>
                  <input
                    type="text"
                    required
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder="e.g. State of Gujarat v. Union of India: Constitutional Limits on Federal Fiscal Powers"
                    className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2.5 text-xs font-bold text-slate-900 focus:border-amber-400 focus:outline-none"
                  />
                </div>

                <div>
                  <label className="block text-xs font-bold uppercase tracking-wider text-slate-700 mb-1">
                    Judgment Summary &amp; Key Holding *
                  </label>
                  <textarea
                    rows={4}
                    required
                    value={summary}
                    onChange={(e) => setSummary(e.target.value)}
                    placeholder="Summarize the core ratio decidendi, judges on the bench, and final legal operative order..."
                    className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-3 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                  />
                </div>

                {/* PDF Uploader */}
                <PDFUploader
                  onUploadSuccess={(meta) => setPdfMeta(meta)}
                  onRemoveAttachment={() => setPdfMeta(null)}
                />

                {/* Paywall Tier */}
                <div>
                  <label className="block text-xs font-bold text-slate-700 mb-1.5">
                    Access Level
                  </label>
                  <div className="grid grid-cols-2 gap-2">
                    <button
                      type="button"
                      onClick={() => setAccessLevel('free')}
                      className={`p-2 rounded-xl border text-center text-xs font-bold transition ${
                        accessLevel === 'free'
                          ? 'border-emerald-500 bg-emerald-50 text-emerald-900'
                          : 'border-slate-200 bg-slate-50 text-slate-600'
                      }`}
                    >
                      Free Public Access
                    </button>
                    <button
                      type="button"
                      onClick={() => setAccessLevel('premium')}
                      className={`p-2 rounded-xl border text-center text-xs font-bold transition ${
                        accessLevel === 'premium'
                          ? 'border-amber-500 bg-amber-50 text-amber-900'
                          : 'border-slate-200 bg-slate-50 text-slate-600'
                      }`}
                    >
                      Premium Paywall
                    </button>
                  </div>
                </div>

                <div className="flex items-center justify-end gap-2 pt-4 border-t border-slate-100">
                  <button
                    type="button"
                    onClick={() => setModalOpen(false)}
                    className="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50 transition"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={creating}
                    className="rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 hover:from-amber-400 hover:to-amber-500 px-5 py-2 text-xs font-extrabold text-slate-950 shadow-md transition disabled:opacity-50"
                  >
                    {creating ? 'Publishing...' : 'Publish Judgment'}
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}
      </CMSLayout>
    </ProtectedRoute>
  );
}
