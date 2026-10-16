'use client';

import React, { useEffect, useState, useRef, Suspense } from 'react';
import ProtectedRoute from '@/components/auth/ProtectedRoute';
import CMSLayout from '@/components/layout/CMSLayout';
import { useAuth } from '@/lib/auth/context';
import apiClient from '@/lib/api/client';
import { useSearchParams } from 'next/navigation';
import {
  Radio,
  Send,
  Edit2,
  Trash2,
  RefreshCw,
  AlertCircle,
  CheckCircle2,
  Loader2,
  Scale,
  Clock,
  User,
  AlertTriangle,
  ChevronDown,
} from 'lucide-react';

type UpdateType = 'argument' | 'judicial_remark' | 'order' | 'verdict' | 'break' | 'adjournment';

interface LiveUpdateItem {
  id: string;
  court_case_id: string;
  sequence_no: number;
  update_type: UpdateType;
  speaker_label?: string | null;
  content: string;
  status: 'normal' | 'corrected' | 'retracted';
  corrects_update_id?: string | null;
  event_time: string;
  created_at: string;
}

function DeskContent() {
  const { user } = useAuth();
  const searchParams = useSearchParams();
  const initialCaseId = searchParams.get('caseId') || '';

  // Cases state
  const [liveCases, setLiveCases] = useState<any[]>([]);
  const [selectedCaseId, setSelectedCaseId] = useState<string>(initialCaseId);
  const [selectedCase, setSelectedCase] = useState<any | null>(null);

  // Form input state
  const [content, setContent] = useState('');
  const [updateType, setUpdateType] = useState<UpdateType>('argument');
  const [speakerLabel, setSpeakerLabel] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitSuccess, setSubmitSuccess] = useState<{ seq: number; id: string } | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);

  // Timeline updates state
  const [updates, setUpdates] = useState<LiveUpdateItem[]>([]);
  const [loadingTimeline, setLoadingTimeline] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);

  // Correction & Retraction modal state
  const [correctingUpdate, setCorrectingUpdate] = useState<LiveUpdateItem | null>(null);
  const [correctionContent, setCorrectionContent] = useState('');
  const [retractingUpdate, setRetractingUpdate] = useState<LiveUpdateItem | null>(null);
  const [retractionReason, setRetractionReason] = useState('Reported in error');
  const [actionLoading, setActionLoading] = useState(false);

  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Fetch live cases list
  const fetchLiveCases = async () => {
    try {
      const { data } = await apiClient.GET('/v1/cases/live');
      const items = (data as any)?.cases || (Array.isArray(data) ? data : []);
      if (Array.isArray(items)) {
        setLiveCases(items);
        if (!selectedCaseId && items.length > 0) {
          setSelectedCaseId(items[0].id);
          setSelectedCase(items[0]);
        } else if (selectedCaseId) {
          const match = items.find((c: any) => c.id === selectedCaseId);
          if (match) setSelectedCase(match);
        }
      }
    } catch {
      // Ignore
    }
  };

  // Fetch updates for selected case
  const fetchUpdates = async (caseId: string) => {
    if (!caseId) return;
    setLoadingTimeline(true);
    try {
      const { data } = await apiClient.GET('/v1/cases/{id}/updates', {
        params: {
          path: { id: caseId },
          query: { limit: 50 },
        },
      });
      const items = (data as any)?.updates || (Array.isArray(data) ? data : []);
      if (Array.isArray(items)) {
        // Sort descending by sequence_no for live feed timeline
        const sorted = [...items].sort((a: any, b: any) => b.sequence_no - a.sequence_no);
        setUpdates(sorted as LiveUpdateItem[]);
      }
    } catch {
      // Ignore
    } finally {
      setLoadingTimeline(false);
    }
  };

  useEffect(() => {
    fetchLiveCases();
  }, []);

  useEffect(() => {
    if (selectedCaseId) {
      const match = liveCases.find((c) => c.id === selectedCaseId);
      if (match) setSelectedCase(match);
      fetchUpdates(selectedCaseId);
    }
  }, [selectedCaseId]);

  // Polling ticker for updates
  useEffect(() => {
    if (!autoRefresh || !selectedCaseId) return;
    const interval = setInterval(() => {
      fetchUpdates(selectedCaseId);
    }, 4000);
    return () => clearInterval(interval);
  }, [autoRefresh, selectedCaseId]);

  // Handle Publish Update with Idempotency Key
  const handlePublish = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!content.trim() || !selectedCaseId) return;

    setIsSubmitting(true);
    setSubmitError(null);
    setSubmitSuccess(null);

    // Generate unique idempotency key to prevent double tap duplicate submissions
    const idempotencyKey = typeof crypto !== 'undefined' && crypto.randomUUID
      ? crypto.randomUUID()
      : `idem_${Date.now()}_${Math.random().toString(36).substring(2, 9)}`;

    try {
      const { data, error, response } = await apiClient.POST('/v1/cms/cases/{id}/updates', {
        params: {
          path: { id: selectedCaseId },
          header: { 'Idempotency-Key': idempotencyKey },
        },
        body: {
          update_type: updateType,
          speaker_label: speakerLabel.trim() ? speakerLabel.trim() : undefined,
          content: content.trim(),
        },
      });

      if (!response.ok || !data) {
        const msg = (error as any)?.message || 'Failed to publish live update';
        setSubmitError(msg);
      } else {
        setSubmitSuccess({ seq: data.sequence_no, id: data.id });
        setContent('');
        setSpeakerLabel('');
        fetchUpdates(selectedCaseId);
        // Refocus textarea for rapid reporting
        setTimeout(() => textareaRef.current?.focus(), 100);
      }
    } catch (err: any) {
      setSubmitError(err.message || 'Network error publishing update');
    } finally {
      setIsSubmitting(false);
    }
  };

  // Submit Correction
  const handleCorrect = async () => {
    if (!correctingUpdate || !correctionContent.trim() || !selectedCaseId) return;
    setActionLoading(true);
    try {
      const { response, error } = await apiClient.POST(
        '/v1/cms/cases/{id}/updates/{updateId}/correct',
        {
          params: {
            path: { id: selectedCaseId, updateId: correctingUpdate.id },
          },
          body: {
            update_type: correctingUpdate.update_type,
            speaker_label: correctingUpdate.speaker_label || undefined,
            content: correctionContent.trim(),
          },
        }
      );
      if (response.ok) {
        setCorrectingUpdate(null);
        setCorrectionContent('');
        fetchUpdates(selectedCaseId);
      } else {
        alert((error as any)?.message || 'Failed to submit correction');
      }
    } catch (err: any) {
      alert(err.message || 'Network error');
    } finally {
      setActionLoading(false);
    }
  };

  // Submit Retraction
  const handleRetract = async () => {
    if (!retractingUpdate || !selectedCaseId) return;
    setActionLoading(true);
    try {
      const { response, error } = await apiClient.POST(
        '/v1/cms/cases/{id}/updates/{updateId}/retract',
        {
          params: {
            path: { id: selectedCaseId, updateId: retractingUpdate.id },
          },
          body: {
            reason: retractionReason.trim() || 'Reported in error',
          },
        }
      );
      if (response.ok) {
        setRetractingUpdate(null);
        fetchUpdates(selectedCaseId);
      } else {
        alert((error as any)?.message || 'Failed to retract update');
      }
    } catch (err: any) {
      alert(err.message || 'Network error');
    } finally {
      setActionLoading(false);
    }
  };

  const getUpdateBadgeStyle = (type: UpdateType) => {
    switch (type) {
      case 'verdict':
        return 'bg-red-500 text-white font-extrabold shadow-sm shadow-red-500/20';
      case 'order':
        return 'bg-blue-600 text-white font-bold';
      case 'argument':
        return 'bg-purple-600 text-white font-semibold';
      case 'judicial_remark':
        return 'bg-amber-600 text-white font-semibold';
      case 'break':
      case 'adjournment':
      default:
        return 'bg-slate-800 text-slate-100 font-medium';
    }
  };

  return (
    <ProtectedRoute allowedRoles={['reporter', 'editor', 'admin']}>
      <CMSLayout title="Live Court Desk" badge="Real-time Reporting">
        {/* Mobile & Desktop Header: Court Case Selector */}
        <div className="bg-slate-900 rounded-2xl border border-slate-800 p-4 sm:p-5 shadow-lg mb-6 text-white">
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
            <div className="flex-1">
              <label htmlFor="case-select" className="block text-xs font-bold uppercase tracking-wider text-amber-400 mb-1.5 flex items-center gap-1.5">
                <Radio className="h-3.5 w-3.5 animate-pulse text-red-400" />
                Select Active Court Hearing
              </label>
              <div className="relative">
                <select
                  id="case-select"
                  value={selectedCaseId}
                  onChange={(e) => setSelectedCaseId(e.target.value)}
                  className="w-full appearance-none rounded-xl bg-slate-950 border border-slate-700 px-4 py-3 text-sm font-semibold text-white focus:border-amber-400 focus:outline-none focus:ring-1 focus:ring-amber-400"
                >
                  {liveCases.length === 0 ? (
                    <option value="">No live court cases active right now</option>
                  ) : (
                    liveCases.map((c) => (
                      <option key={c.id} value={c.id}>
                        [{c.court_name}] {c.case_number}: {c.title}
                      </option>
                    ))
                  )}
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-3 text-slate-400">
                  <ChevronDown className="h-4 w-4" />
                </div>
              </div>
            </div>

            {selectedCase && (
              <div className="flex items-center gap-3 pt-2 md:pt-0 border-t md:border-t-0 border-slate-800">
                <div className="text-right hidden sm:block">
                  <span className="text-[11px] text-slate-400 uppercase tracking-wider font-semibold block">Latest Sequence</span>
                  <span className="text-xl font-black text-amber-400">#{selectedCase.last_sequence_no || 0}</span>
                </div>
                <button
                  type="button"
                  onClick={() => fetchUpdates(selectedCaseId)}
                  disabled={loadingTimeline}
                  className="p-3 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 transition"
                  title="Manual refresh"
                >
                  <RefreshCw className={`h-4 w-4 ${loadingTimeline ? 'animate-spin' : ''}`} />
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Main 2-Column Responsive Layout (Mobile: Stacked form on top, timeline below) */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
          {/* LEFT: Rapid Publishing Form Card */}
          <div className="lg:col-span-6 xl:col-span-5">
            <div className="sticky top-20 rounded-2xl bg-white border border-slate-200/90 shadow-md p-5 sm:p-6">
              <div className="flex items-center justify-between pb-4 mb-4 border-b border-slate-100">
                <h3 className="font-extrabold text-slate-900 text-lg tracking-tight flex items-center gap-2">
                  <Send className="h-5 w-5 text-amber-600" />
                  Post Live Update
                </h3>
                <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 border border-emerald-200">
                  Idempotent Submit
                </span>
              </div>

              {/* Status alerts */}
              {submitSuccess && (
                <div className="mb-4 flex items-center gap-2.5 rounded-xl bg-emerald-50 border border-emerald-200 p-3 text-sm text-emerald-800 font-medium animate-in fade-in">
                  <CheckCircle2 className="h-5 w-5 text-emerald-600 shrink-0" />
                  <span>
                    Successfully published update <strong className="font-bold">#{submitSuccess.seq}</strong>!
                  </span>
                </div>
              )}

              {submitError && (
                <div className="mb-4 flex items-start gap-2.5 rounded-xl bg-red-50 border border-red-200 p-3 text-sm text-red-800 font-medium">
                  <AlertCircle className="h-5 w-5 text-red-600 shrink-0 mt-0.5" />
                  <div className="flex-1">
                    <p>{submitError}</p>
                    <button
                      type="button"
                      onClick={handlePublish}
                      className="mt-1.5 text-xs font-bold underline text-red-900"
                    >
                      Retry Publish
                    </button>
                  </div>
                </div>
              )}

              <form onSubmit={handlePublish} className="space-y-4">
                {/* Update Type Selector */}
                <div>
                  <label className="block text-xs font-bold uppercase tracking-wider text-slate-600 mb-1.5">
                    Update Type
                  </label>
                  <div className="grid grid-cols-3 gap-1.5 sm:gap-2">
                    {[
                      { type: 'argument', label: 'Argument' },
                      { type: 'judicial_remark', label: 'Bench Remark' },
                      { type: 'order', label: 'Order' },
                      { type: 'verdict', label: 'Verdict' },
                      { type: 'break', label: 'Recess' },
                      { type: 'adjournment', label: 'Adjourn' },
                    ].map((item) => (
                      <button
                        key={item.type}
                        type="button"
                        onClick={() => setUpdateType(item.type as UpdateType)}
                        className={`rounded-xl py-2 px-2 text-xs font-bold uppercase tracking-wider transition ${
                          updateType === item.type
                            ? 'bg-slate-900 text-white shadow-sm ring-2 ring-amber-400'
                            : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                        }`}
                      >
                        {item.label}
                      </button>
                    ))}
                  </div>
                </div>

                {/* Speaker Label (Optional) */}
                <div>
                  <label htmlFor="speaker" className="block text-xs font-bold uppercase tracking-wider text-slate-600 mb-1.5">
                    Speaker Label <span className="font-normal text-slate-400">(Optional)</span>
                  </label>
                  <div className="relative rounded-xl">
                    <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-slate-400">
                      <User className="h-4 w-4" />
                    </div>
                    <input
                      id="speaker"
                      type="text"
                      value={speakerLabel}
                      onChange={(e) => setSpeakerLabel(e.target.value)}
                      placeholder="e.g. CJI, SG Tushar Mehta, Bench"
                      className="block w-full rounded-xl border border-slate-200 bg-slate-50 pl-9 pr-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:bg-white focus:border-amber-500 focus:outline-none focus:ring-1 focus:ring-amber-500"
                    />
                  </div>
                </div>

                {/* Main Large Textbox */}
                <div>
                  <label htmlFor="content" className="block text-xs font-bold uppercase tracking-wider text-slate-600 mb-1.5">
                    Courtroom Live Text <span className="text-red-500">*</span>
                  </label>
                  <textarea
                    id="content"
                    ref={textareaRef}
                    required
                    rows={5}
                    value={content}
                    onChange={(e) => setContent(e.target.value)}
                    placeholder="Type the courtroom update verbatim or summarized..."
                    className="block w-full rounded-xl border border-slate-200 bg-slate-50 p-3 text-base sm:text-sm text-slate-900 placeholder:text-slate-400 focus:bg-white focus:border-amber-500 focus:outline-none focus:ring-2 focus:ring-amber-500/20"
                  />
                  <div className="flex justify-between items-center mt-1 text-[11px] text-slate-400">
                    <span>Markdown formatting supported</span>
                    <span>{content.length} characters</span>
                  </div>
                </div>

                {/* Big Publish Button */}
                <button
                  type="submit"
                  disabled={isSubmitting || !content.trim() || !selectedCaseId}
                  className="w-full flex items-center justify-center gap-2 rounded-2xl bg-gradient-to-r from-amber-500 to-amber-600 py-4 px-6 text-base font-extrabold text-slate-950 shadow-lg shadow-amber-500/25 hover:from-amber-400 hover:to-amber-500 focus:outline-none focus:ring-4 focus:ring-amber-400/50 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {isSubmitting ? (
                    <>
                      <Loader2 className="h-5 w-5 animate-spin" />
                      Publishing to Ticker...
                    </>
                  ) : (
                    <>
                      <Send className="h-5 w-5 text-slate-950" />
                      Publish Live Update
                    </>
                  )}
                </button>
              </form>
            </div>
          </div>

          {/* RIGHT: Live Timeline with Correct / Retract Actions */}
          <div className="lg:col-span-6 xl:col-span-7">
            <div className="rounded-2xl bg-white border border-slate-200/90 shadow-md p-5 sm:p-6 min-h-[500px]">
              <div className="flex items-center justify-between pb-4 mb-4 border-b border-slate-100">
                <div className="flex items-center gap-2">
                  <Clock className="h-5 w-5 text-slate-600" />
                  <h3 className="font-extrabold text-slate-900 text-lg tracking-tight">
                    Live Timeline Feed
                  </h3>
                </div>
                <div className="flex items-center gap-3">
                  <label className="text-xs text-slate-600 flex items-center gap-1.5 cursor-pointer font-medium">
                    <input
                      type="checkbox"
                      checked={autoRefresh}
                      onChange={(e) => setAutoRefresh(e.target.checked)}
                      className="rounded border-slate-300 text-amber-600 focus:ring-amber-500"
                    />
                    Auto-pull (4s)
                  </label>
                  <span className="text-xs font-bold text-slate-400 bg-slate-100 px-2 py-0.5 rounded-full">
                    {updates.length} updates
                  </span>
                </div>
              </div>

              {/* Updates stream list */}
              {loadingTimeline && updates.length === 0 ? (
                <div className="py-20 text-center text-slate-400">
                  <Loader2 className="h-8 w-8 animate-spin mx-auto mb-3 text-amber-500" />
                  <p className="text-sm font-semibold">Streaming courtroom timeline...</p>
                </div>
              ) : updates.length === 0 ? (
                <div className="py-20 text-center text-slate-400">
                  <Scale className="h-10 w-10 text-slate-300 mx-auto mb-3" />
                  <p className="text-base font-bold text-slate-700">No updates posted yet</p>
                  <p className="text-xs text-slate-400 mt-1">Use the panel on the left to post the first courtroom update.</p>
                </div>
              ) : (
                <div className="space-y-4 max-h-[750px] overflow-y-auto pr-1">
                  {updates.map((item) => (
                    <div
                      key={item.id}
                      className={`relative rounded-xl border p-4 transition-all ${
                        item.status === 'retracted'
                          ? 'bg-slate-50 border-dashed border-red-200 opacity-60'
                          : item.status === 'corrected'
                          ? 'bg-amber-50/40 border-amber-200/80 shadow-sm'
                          : 'bg-white border-slate-200/80 shadow-sm hover:border-slate-300'
                      }`}
                    >
                      {/* Card Header: Sequence, Type badge, Speaker, Timestamp */}
                      <div className="flex items-center justify-between gap-2 mb-2 flex-wrap">
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-xs font-black text-slate-900 bg-slate-100 px-2 py-0.5 rounded">
                            #{item.sequence_no}
                          </span>
                          <span className={`text-[10px] uppercase tracking-wider px-2 py-0.5 rounded ${getUpdateBadgeStyle(item.update_type)}`}>
                            {item.update_type}
                          </span>
                          {item.speaker_label && (
                            <span className="text-xs font-bold text-slate-800 bg-slate-100/80 px-2 py-0.5 rounded border border-slate-200">
                              {item.speaker_label}
                            </span>
                          )}
                          {item.status === 'retracted' && (
                            <span className="text-[10px] font-bold uppercase tracking-wider bg-red-100 text-red-700 px-1.5 py-0.5 rounded">
                              Retracted
                            </span>
                          )}
                          {item.status === 'corrected' && (
                            <span className="text-[10px] font-bold uppercase tracking-wider bg-amber-100 text-amber-800 px-1.5 py-0.5 rounded">
                              Corrected
                            </span>
                          )}
                        </div>
                        <span className="text-[11px] text-slate-400 font-medium">
                          {new Date(item.created_at).toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                        </span>
                      </div>

                      {/* Content */}
                      <p className={`text-sm text-slate-800 whitespace-pre-wrap leading-relaxed ${item.status === 'retracted' ? 'line-through text-slate-400' : ''}`}>
                        {item.content}
                      </p>

                      {/* Action buttons (Correct & Retract) */}
                      {item.status !== 'retracted' && (
                        <div className="mt-3 pt-2.5 border-t border-slate-100 flex items-center justify-end gap-2">
                          <button
                            type="button"
                            onClick={() => {
                              setCorrectingUpdate(item);
                              setCorrectionContent(item.content);
                            }}
                            className="inline-flex items-center gap-1 text-xs font-semibold text-slate-600 hover:text-amber-600 px-2.5 py-1 rounded-lg hover:bg-amber-50 transition"
                          >
                            <Edit2 className="h-3.5 w-3.5" />
                            Correct
                          </button>
                          <button
                            type="button"
                            onClick={() => setRetractingUpdate(item)}
                            className="inline-flex items-center gap-1 text-xs font-semibold text-slate-600 hover:text-red-600 px-2.5 py-1 rounded-lg hover:bg-red-50 transition"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                            Retract
                          </button>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Modal: Correct Update */}
        {correctingUpdate && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm animate-in fade-in">
            <div className="bg-white rounded-2xl max-w-lg w-full p-6 shadow-2xl border border-slate-200">
              <h4 className="text-lg font-bold text-slate-900 mb-2 flex items-center gap-2">
                <Edit2 className="h-5 w-5 text-amber-600" />
                Correct Update #{correctingUpdate.sequence_no}
              </h4>
              <p className="text-xs text-slate-500 mb-4">
                Corrections are append-only. A new correction update will be published referencing sequence #{correctingUpdate.sequence_no}.
              </p>
              <textarea
                rows={4}
                value={correctionContent}
                onChange={(e) => setCorrectionContent(e.target.value)}
                className="w-full rounded-xl border border-slate-300 p-3 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-amber-500 mb-4"
              />
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setCorrectingUpdate(null)}
                  disabled={actionLoading}
                  className="px-4 py-2 rounded-xl text-xs font-semibold text-slate-600 hover:bg-slate-100"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleCorrect}
                  disabled={actionLoading || !correctionContent.trim()}
                  className="px-4 py-2 rounded-xl text-xs font-bold bg-amber-500 hover:bg-amber-600 text-slate-950 flex items-center gap-1.5 shadow"
                >
                  {actionLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Publish Correction'}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Modal: Retract Update */}
        {retractingUpdate && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm animate-in fade-in">
            <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-2xl border border-slate-200">
              <div className="rounded-full bg-red-100 text-red-600 p-3 w-12 h-12 flex items-center justify-center mx-auto mb-4">
                <AlertTriangle className="h-6 w-6" />
              </div>
              <h4 className="text-lg font-bold text-slate-900 text-center mb-1">
                Retract Update #{retractingUpdate.sequence_no}?
              </h4>
              <p className="text-xs text-slate-500 text-center mb-4">
                Are you sure you want to retract this update? The retraction will be logged in the public ledger and marked as retracted.
              </p>
              <div className="mb-5">
                <label className="block text-xs font-bold uppercase tracking-wider text-slate-700 mb-1.5 text-left">
                  Retraction Reason
                </label>
                <input
                  type="text"
                  value={retractionReason}
                  onChange={(e) => setRetractionReason(e.target.value)}
                  placeholder="e.g. Reported in error / Misattributed counsel quote"
                  className="w-full rounded-xl border border-slate-300 p-2.5 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-red-500"
                />
              </div>
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setRetractingUpdate(null)}
                  disabled={actionLoading}
                  className="w-full px-4 py-2.5 rounded-xl text-xs font-semibold text-slate-600 hover:bg-slate-100"
                >
                  Keep Update
                </button>
                <button
                  type="button"
                  onClick={handleRetract}
                  disabled={actionLoading}
                  className="w-full px-4 py-2.5 rounded-xl text-xs font-bold bg-red-600 hover:bg-red-700 text-white flex items-center justify-center gap-1.5 shadow"
                >
                  {actionLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Confirm Retract'}
                </button>
              </div>
            </div>
          </div>
        )}
      </CMSLayout>
    </ProtectedRoute>
  );
}

export default function LiveCourtDeskPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen flex items-center justify-center bg-slate-950 text-white">
          <Loader2 className="h-8 w-8 animate-spin text-amber-500" />
        </div>
      }
    >
      <DeskContent />
    </Suspense>
  );
}
