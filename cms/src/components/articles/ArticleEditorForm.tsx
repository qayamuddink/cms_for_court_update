'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import apiClient from '@/lib/api/client';
import { useAuth } from '@/lib/auth/context';
import TipTapEditor from '@/components/editor/TipTapEditor';
import PDFUploader, { UploadedPDFMeta } from './PDFUploader';
import {
  Save,
  Send,
  CheckCircle2,
  RotateCcw,
  Archive,
  ArrowLeft,
  AlertTriangle,
  Loader2,
  Sparkles,
  ShieldCheck,
  Scale,
  FileText,
} from 'lucide-react';

export interface ArticleData {
  id?: string;
  title: string;
  subtitle?: string | null;
  summary: string;
  body_blocks?: any[] | null;
  article_type: 'news' | 'commentary' | 'editorial' | 'judgment_summary';
  access_level: 'free' | 'premium';
  category: string;
  tags?: string[] | null;
  court_case_id?: string | null;
  court_name?: string | null;
  citation?: string | null;
  status?: 'draft' | 'in_review' | 'published' | 'archived';
  version?: number;
  pdf_storage_key?: string | null;
  pdf_size_bytes?: number | null;
  pdf_checksum?: string | null;
}

interface ArticleEditorFormProps {
  initialData?: ArticleData | null;
  isNew?: boolean;
}

export default function ArticleEditorForm({ initialData, isNew = false }: ArticleEditorFormProps) {
  const router = useRouter();
  const { user, hasRole } = useAuth();

  const [articleId, setArticleId] = useState<string | undefined>(initialData?.id);
  const [version, setVersion] = useState<number>(initialData?.version || 1);
  const [status, setStatus] = useState<string>(initialData?.status || 'draft');

  // Form Fields
  const [title, setTitle] = useState(initialData?.title || '');
  const [subtitle, setSubtitle] = useState(initialData?.subtitle || '');
  const [summary, setSummary] = useState(initialData?.summary || '');
  const [bodyBlocks, setBodyBlocks] = useState<any[]>(initialData?.body_blocks || []);
  const [articleType, setArticleType] = useState<ArticleData['article_type']>(
    initialData?.article_type || 'news'
  );
  const [accessLevel, setAccessLevel] = useState<'free' | 'premium'>(
    initialData?.access_level || 'free'
  );
  const [category, setCategory] = useState(initialData?.category || 'Supreme Court');
  const [tagsInput, setTagsInput] = useState((initialData?.tags || []).join(', '));
  const [courtCaseId, setCourtCaseId] = useState(initialData?.court_case_id || '');
  const [courtName, setCourtName] = useState(initialData?.court_name || 'Supreme Court of India');
  const [citation, setCitation] = useState(initialData?.citation || '');

  // Attachment state
  const [attachment, setAttachment] = useState<{
    storage_key?: string | null;
    size_bytes?: number | null;
    checksum?: string | null;
    filename?: string | null;
  } | null>(
    initialData?.pdf_storage_key
      ? {
          storage_key: initialData.pdf_storage_key,
          size_bytes: initialData.pdf_size_bytes,
          checksum: initialData.pdf_checksum,
        }
      : null
  );

  // Live cases for linked picker
  const [liveCases, setLiveCases] = useState<any[]>([]);

  // State flags
  const [saving, setSaving] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [conflictModalOpen, setConflictModalOpen] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(
    null
  );

  // Send Back Modal
  const [sendBackOpen, setSendBackOpen] = useState(false);
  const [sendBackComment, setSendBackComment] = useState('Needs minor factual corrections.');

  useEffect(() => {
    // Fetch live cases for linking
    const fetchCases = async () => {
      try {
        const { data } = await apiClient.GET('/v1/cases/live');
        const items = (data as any)?.cases || (Array.isArray(data) ? data : []);
        setLiveCases(items);
      } catch {
        // Ignore
      }
    };
    fetchCases();
  }, []);

  // Save or Update Draft
  const handleSave = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!title.trim()) {
      setStatusMessage({ text: 'Article title is required.', type: 'error' });
      return;
    }

    setSaving(true);
    setStatusMessage(null);

    const tags = tagsInput
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);

    try {
      if (isNew || !articleId) {
        // Create new draft
        const { data, error, response } = await apiClient.POST('/v1/cms/articles', {
          body: {
            title: title.trim(),
            subtitle: subtitle.trim() ? subtitle.trim() : undefined,
            summary: summary.trim(),
            body_blocks: bodyBlocks.length > 0 ? bodyBlocks : undefined,
            article_type: articleType,
            access_level: accessLevel,
            category: category.trim() || 'General',
            tags: tags.length > 0 ? tags : undefined,
            court_case_id: courtCaseId.trim() ? courtCaseId.trim() : undefined,
            court_name: courtName.trim() ? courtName.trim() : undefined,
            citation: citation.trim() ? citation.trim() : undefined,
          },
        });

        if (!response.ok || !data) {
          throw new Error((error as any)?.message || 'Failed to create article draft');
        }

        setArticleId(data.id);
        setVersion(data.version);
        setStatus(data.status);
        setStatusMessage({ text: 'Draft created successfully!', type: 'success' });
        router.replace(`/articles/${data.id}/edit`);
      } else {
        // Update existing draft with optimistic concurrency expected_version
        const { data, error, response } = await apiClient.PATCH('/v1/cms/articles/{id}', {
          params: { path: { id: articleId } },
          body: {
            expected_version: version,
            title: title.trim(),
            subtitle: subtitle.trim() ? subtitle.trim() : undefined,
            summary: summary.trim(),
            body_blocks: bodyBlocks.length > 0 ? bodyBlocks : undefined,
            article_type: articleType,
            access_level: accessLevel,
            category: category.trim() || 'General',
            tags: tags.length > 0 ? tags : undefined,
            court_case_id: courtCaseId.trim() ? courtCaseId.trim() : undefined,
            court_name: courtName.trim() ? courtName.trim() : undefined,
            citation: citation.trim() ? citation.trim() : undefined,
          },
        });

        if (response.status === 409) {
          // Version conflict
          setConflictModalOpen(true);
          return;
        }

        if (!response.ok || !data) {
          throw new Error((error as any)?.message || 'Failed to update article draft');
        }

        setVersion(data.version);
        setStatus(data.status);
        setStatusMessage({ text: `Article saved (v${data.version})!`, type: 'success' });
      }
    } catch (err: any) {
      setStatusMessage({ text: err.message || 'Error saving article', type: 'error' });
    } finally {
      setSaving(false);
    }
  };

  // Submit for Review
  const handleSubmitForReview = async () => {
    if (!articleId) return;
    setActionLoading(true);
    setStatusMessage(null);
    try {
      const { data, error, response } = await apiClient.POST('/v1/cms/articles/{id}/submit', {
        params: { path: { id: articleId } },
      });
      if (response.ok && data) {
        setStatus(data.status);
        setStatusMessage({ text: 'Article submitted to editor review queue.', type: 'success' });
      } else {
        setStatusMessage({ text: (error as any)?.message || 'Failed to submit article', type: 'error' });
      }
    } catch (err: any) {
      setStatusMessage({ text: err.message || 'Network error', type: 'error' });
    } finally {
      setActionLoading(false);
    }
  };

  // Publish Article
  const handlePublish = async () => {
    if (!articleId) return;
    setActionLoading(true);
    setStatusMessage(null);
    try {
      const { data, error, response } = await apiClient.POST('/v1/cms/articles/{id}/publish', {
        params: { path: { id: articleId } },
      });
      if (response.ok && data) {
        setStatus(data.status);
        setStatusMessage({ text: 'Article published live successfully!', type: 'success' });
      } else {
        setStatusMessage({ text: (error as any)?.message || 'Failed to publish article', type: 'error' });
      }
    } catch (err: any) {
      setStatusMessage({ text: err.message || 'Network error', type: 'error' });
    } finally {
      setActionLoading(false);
    }
  };

  // Archive Article
  const handleArchive = async () => {
    if (!articleId) return;
    setActionLoading(true);
    setStatusMessage(null);
    try {
      const { data, error, response } = await apiClient.POST('/v1/cms/articles/{id}/archive', {
        params: { path: { id: articleId } },
      });
      if (response.ok && data) {
        setStatus(data.status);
        setStatusMessage({ text: 'Article archived.', type: 'success' });
      } else {
        setStatusMessage({ text: (error as any)?.message || 'Failed to archive article', type: 'error' });
      }
    } catch (err: any) {
      setStatusMessage({ text: err.message || 'Network error', type: 'error' });
    } finally {
      setActionLoading(false);
    }
  };

  // Send Back to Reporter
  const handleSendBack = async () => {
    if (!articleId) return;
    setActionLoading(true);
    try {
      const { data, error, response } = await apiClient.POST('/v1/cms/articles/{id}/send-back', {
        params: { path: { id: articleId } },
        body: { comment: sendBackComment.trim() },
      });
      if (response.ok && data) {
        setSendBackOpen(false);
        setStatus(data.status);
        setStatusMessage({ text: 'Article sent back to reporter draft.', type: 'success' });
      } else {
        alert((error as any)?.message || 'Failed to send back article');
      }
    } catch (err: any) {
      alert(err.message || 'Network error');
    } finally {
      setActionLoading(false);
    }
  };

  const handleUploadSuccess = (meta: UploadedPDFMeta) => {
    setAttachment(meta);
    setStatusMessage({ text: `Judgment PDF attached: ${meta.filename}`, type: 'success' });
  };

  return (
    <div className="space-y-6">
      {/* Top Navigation & Workflow Action Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-white border border-slate-200 rounded-2xl p-4 shadow-sm">
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => router.push('/articles')}
            className="rounded-xl border border-slate-200 bg-slate-50 hover:bg-slate-100 p-2 text-slate-600 transition"
            title="Back to articles"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div>
            <div className="flex items-center gap-2">
              <span className="text-xs uppercase tracking-wider font-extrabold px-2.5 py-0.5 rounded-full bg-slate-100 text-slate-800 border border-slate-200">
                {status.replace('_', ' ')}
              </span>
              {articleId && (
                <span className="font-mono text-xs font-semibold text-slate-400 bg-slate-50 px-2 py-0.5 rounded border border-slate-200">
                  v{version}
                </span>
              )}
            </div>
            <p className="text-[11px] text-slate-400 mt-0.5">
              {isNew ? 'New Editorial Draft' : `Editing Article #${articleId?.substring(0, 8)}...`}
            </p>
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex items-center gap-2 flex-wrap">
          {/* Save Draft */}
          <button
            type="button"
            onClick={handleSave}
            disabled={saving || actionLoading}
            className="inline-flex items-center gap-1.5 rounded-xl bg-slate-900 hover:bg-slate-800 px-4 py-2 text-xs font-bold text-white transition disabled:opacity-50"
          >
            {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            Save Draft
          </button>

          {/* Submit for Review */}
          {status === 'draft' && articleId && (
            <button
              type="button"
              onClick={handleSubmitForReview}
              disabled={actionLoading || saving}
              className="inline-flex items-center gap-1.5 rounded-xl bg-blue-600 hover:bg-blue-700 px-4 py-2 text-xs font-bold text-white transition shadow-sm disabled:opacity-50"
            >
              <Send className="h-4 w-4" />
              Submit for Review
            </button>
          )}

          {/* Editor/Admin Actions when in_review */}
          {status === 'in_review' && hasRole('editor', 'admin') && (
            <>
              <button
                type="button"
                onClick={handlePublish}
                disabled={actionLoading || saving}
                className="inline-flex items-center gap-1.5 rounded-xl bg-emerald-500 hover:bg-emerald-600 px-4 py-2 text-xs font-extrabold text-slate-950 transition shadow-sm disabled:opacity-50"
              >
                <CheckCircle2 className="h-4 w-4" />
                Publish Live
              </button>
              <button
                type="button"
                onClick={() => setSendBackOpen(true)}
                disabled={actionLoading || saving}
                className="inline-flex items-center gap-1.5 rounded-xl border border-amber-300 bg-amber-50 hover:bg-amber-100 px-4 py-2 text-xs font-bold text-amber-900 transition disabled:opacity-50"
              >
                <RotateCcw className="h-4 w-4" />
                Send Back
              </button>
            </>
          )}

          {/* Archive when published */}
          {status === 'published' && hasRole('editor', 'admin') && (
            <button
              type="button"
              onClick={handleArchive}
              disabled={actionLoading || saving}
              className="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 px-3.5 py-2 text-xs font-semibold text-slate-700 transition disabled:opacity-50"
            >
              <Archive className="h-4 w-4" />
              Archive
            </button>
          )}
        </div>
      </div>

      {/* Notifications */}
      {statusMessage && (
        <div
          className={`flex items-center justify-between rounded-xl border p-4 text-xs font-bold transition ${
            statusMessage.type === 'success'
              ? 'bg-emerald-50 border-emerald-200 text-emerald-800'
              : 'bg-red-50 border-red-200 text-red-800'
          }`}
        >
          <span>{statusMessage.text}</span>
          <button
            type="button"
            onClick={() => setStatusMessage(null)}
            className="underline ml-4"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Main Grid: Left Editor & Right Sidebar */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* LEFT COLUMN: Main Writing Surface */}
        <div className="lg:col-span-8 space-y-6">
          {/* Title & Subtitle */}
          <div className="rounded-2xl bg-white border border-slate-200 p-5 shadow-sm space-y-4">
            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-slate-700 mb-1.5">
                Headline / Title *
              </label>
              <input
                type="text"
                required
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="e.g. Supreme Court Holds Right to Privacy is a Fundamental Right Under Article 21"
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-3 text-base sm:text-lg font-extrabold text-slate-900 placeholder:text-slate-400 focus:border-amber-400 focus:outline-none focus:ring-1 focus:ring-amber-400"
              />
            </div>

            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-slate-700 mb-1.5">
                Subtitle / Deck
              </label>
              <input
                type="text"
                value={subtitle}
                onChange={(e) => setSubtitle(e.target.value)}
                placeholder="e.g. 9-judge constitution bench unanimously overrules MP Sharma and Kharak Singh judgments"
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2.5 text-xs text-slate-800 placeholder:text-slate-400 focus:border-amber-400 focus:outline-none focus:ring-1 focus:ring-amber-400"
              />
            </div>

            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-slate-700 mb-1.5">
                Summary / TL;DR (App Feed Preview) *
              </label>
              <textarea
                rows={3}
                required
                value={summary}
                onChange={(e) => setSummary(e.target.value)}
                placeholder="A concise summary for notifications, mobile feed preview, and RSS..."
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-3 text-xs text-slate-800 placeholder:text-slate-400 focus:border-amber-400 focus:outline-none focus:ring-1 focus:ring-amber-400"
              />
            </div>
          </div>

          {/* TipTap Rich Text Editor */}
          <div className="space-y-2">
            <label className="block text-xs font-bold uppercase tracking-wider text-slate-700">
              Article Body (TipTap Rich Text)
            </label>
            <TipTapEditor
              content={bodyBlocks}
              onChange={(blocks) => setBodyBlocks(blocks)}
              placeholder="Draft your courtroom report, legal analysis, case ratio, and judgment excerpts here..."
            />
          </div>

          {/* Judgment PDF Attachment Uploader */}
          <PDFUploader
            currentAttachment={attachment}
            articleId={articleId}
            onUploadSuccess={handleUploadSuccess}
            onRemoveAttachment={() => setAttachment(null)}
          />
        </div>

        {/* RIGHT COLUMN: Editorial Settings Sidebar */}
        <div className="lg:col-span-4 space-y-6">
          {/* Access Level Paywall Toggle */}
          <div className="rounded-2xl bg-white border border-slate-200 p-5 shadow-sm space-y-3">
            <h4 className="text-xs font-extrabold uppercase tracking-wider text-slate-700 flex items-center gap-1.5">
              <Sparkles className="h-4 w-4 text-amber-500" />
              Paywall Access Tier
            </h4>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => setAccessLevel('free')}
                className={`p-3 rounded-xl border text-center transition ${
                  accessLevel === 'free'
                    ? 'border-emerald-500 bg-emerald-50 text-emerald-900 font-extrabold shadow-sm'
                    : 'border-slate-200 bg-slate-50/60 text-slate-600 hover:bg-slate-100 font-medium'
                }`}
              >
                <div className="text-xs font-bold">Free Access</div>
                <div className="text-[10px] text-slate-500 mt-0.5">Public to all readers</div>
              </button>
              <button
                type="button"
                onClick={() => setAccessLevel('premium')}
                className={`p-3 rounded-xl border text-center transition ${
                  accessLevel === 'premium'
                    ? 'border-amber-500 bg-amber-50 text-amber-900 font-extrabold shadow-sm'
                    : 'border-slate-200 bg-slate-50/60 text-slate-600 hover:bg-slate-100 font-medium'
                }`}
              >
                <div className="text-xs font-bold text-amber-900 flex items-center justify-center gap-1">
                  <Sparkles className="h-3 w-3 text-amber-600" />
                  Premium
                </div>
                <div className="text-[10px] text-slate-500 mt-0.5">Subscribers only</div>
              </button>
            </div>
          </div>

          {/* Classification & Metadata */}
          <div className="rounded-2xl bg-white border border-slate-200 p-5 shadow-sm space-y-4">
            <h4 className="text-xs font-extrabold uppercase tracking-wider text-slate-700">
              Taxonomy &amp; Metadata
            </h4>

            <div>
              <label className="block text-xs font-bold text-slate-700 mb-1">
                Article Type
              </label>
              <select
                value={articleType}
                onChange={(e) => setArticleType(e.target.value as any)}
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-semibold text-slate-800 focus:border-amber-400 focus:outline-none"
              >
                <option value="news">News Report</option>
                <option value="commentary">Legal Commentary</option>
                <option value="editorial">Editorial Opinion</option>
                <option value="judgment_summary">Judgment Summary</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-bold text-slate-700 mb-1">
                Category
              </label>
              <select
                value={category}
                onChange={(e) => setCategory(e.target.value)}
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-semibold text-slate-800 focus:border-amber-400 focus:outline-none"
              >
                <option value="Supreme Court">Supreme Court</option>
                <option value="High Courts">High Courts</option>
                <option value="Constitutional Law">Constitutional Law</option>
                <option value="Criminal Law">Criminal Law</option>
                <option value="Corporate Law">Corporate Law</option>
                <option value="Human Rights">Human Rights</option>
                <option value="Taxation">Taxation</option>
                <option value="Arbitration">Arbitration</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-bold text-slate-700 mb-1">
                Tags (Comma separated)
              </label>
              <input
                type="text"
                value={tagsInput}
                onChange={(e) => setTagsInput(e.target.value)}
                placeholder="Article 21, Right to Privacy, CJI"
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs text-slate-800 placeholder:text-slate-400 focus:border-amber-400 focus:outline-none"
              />
            </div>
          </div>

          {/* Court & Citation Linker */}
          <div className="rounded-2xl bg-white border border-slate-200 p-5 shadow-sm space-y-4">
            <h4 className="text-xs font-extrabold uppercase tracking-wider text-slate-700 flex items-center gap-1.5">
              <Scale className="h-4 w-4 text-slate-600" />
              Court &amp; Case Linkage
            </h4>

            <div>
              <label className="block text-xs font-bold text-slate-700 mb-1">
                Linked Active Live Case
              </label>
              <select
                value={courtCaseId}
                onChange={(e) => setCourtCaseId(e.target.value)}
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-semibold text-slate-800 focus:border-amber-400 focus:outline-none"
              >
                <option value="">-- No linked court case --</option>
                {liveCases.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.case_number ? `${c.case_number} - ` : ''}{c.title}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-xs font-bold text-slate-700 mb-1">
                Court Name
              </label>
              <input
                type="text"
                value={courtName}
                onChange={(e) => setCourtName(e.target.value)}
                placeholder="Supreme Court of India"
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
              />
            </div>

            <div>
              <label className="block text-xs font-bold text-slate-700 mb-1">
                Official Legal Citation
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
        </div>
      </div>

      {/* Concurrent Edit Conflict Modal (409 Conflict) */}
      {conflictModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/75 backdrop-blur-sm p-4">
          <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl border border-red-200">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-red-100 text-red-600 mb-3">
              <AlertTriangle className="h-6 w-6 stroke-[2.2]" />
            </div>
            <h3 className="text-base font-extrabold text-slate-900 mb-1">
              Concurrent Edit Conflict Detected
            </h3>
            <p className="text-xs text-slate-500 mb-4 leading-relaxed">
              Another editor has updated this article while you were editing (Version mismatch). To prevent accidental overwriting of editorial changes, your save was rejected by the server (HTTP 409).
            </p>
            <div className="flex items-center justify-end gap-2">
              <button
                type="button"
                onClick={() => setConflictModalOpen(false)}
                className="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50 transition"
              >
                Keep My Changes
              </button>
              <button
                type="button"
                onClick={() => window.location.reload()}
                className="rounded-xl bg-red-600 hover:bg-red-700 px-4 py-2 text-xs font-extrabold text-white transition"
              >
                Reload Latest Version
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Send Back with Feedback Modal */}
      {sendBackOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 backdrop-blur-sm p-4">
          <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl border border-slate-200">
            <h3 className="text-base font-extrabold text-slate-900 mb-2">
              Send Back to Reporter
            </h3>
            <p className="text-xs text-slate-500 mb-4">
              Return this article to draft status with specific editorial comments.
            </p>

            <div className="mb-4">
              <label className="block text-xs font-bold uppercase tracking-wider text-slate-700 mb-1">
                Editorial Review Notes
              </label>
              <textarea
                rows={3}
                value={sendBackComment}
                onChange={(e) => setSendBackComment(e.target.value)}
                className="w-full rounded-xl border border-slate-300 p-3 text-xs text-slate-800 focus:border-amber-400 focus:outline-none focus:ring-1 focus:ring-amber-400"
                placeholder="Specify required corrections or missing details..."
              />
            </div>

            <div className="flex items-center justify-end gap-2">
              <button
                type="button"
                onClick={() => setSendBackOpen(false)}
                className="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50 transition"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleSendBack}
                disabled={actionLoading}
                className="rounded-xl bg-amber-500 hover:bg-amber-600 px-4 py-2 text-xs font-extrabold text-slate-950 transition disabled:opacity-50"
              >
                {actionLoading ? 'Sending...' : 'Confirm Return'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
