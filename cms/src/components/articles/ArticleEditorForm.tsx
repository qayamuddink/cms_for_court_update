'use client';

import React, { useState, useEffect, useMemo } from 'react';
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
  Calendar,
  Clock,
  History,
  Trash2,
  Globe,
  Image as ImageIcon,
  BookOpen,
  FolderTree,
  X,
  Eye,
  RefreshCw,
  Layers,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';

export type ArticleType =
  | 'news'
  | 'commentary'
  | 'editorial'
  | 'judgment_summary'
  | 'top_story'
  | 'column'
  | 'opinion'
  | 'interview'
  | 'round_up'
  | 'digest'
  | 'know_the_law'
  | 'book_review'
  | 'job_update'
  | 'event'
  | 'video'
  | 'cartoon';

export interface ArticleData {
  id?: string;
  title: string;
  subtitle?: string | null;
  summary: string;
  body_blocks?: any[] | null;
  article_type: ArticleType;
  access_level: 'free' | 'premium';
  category: string;
  section_id?: string | null;
  section_name?: string | null;
  section_slug?: string | null;
  featured_image_key?: string | null;
  featured_image_alt?: string | null;
  featured_image_caption?: string | null;
  meta_title?: string | null;
  meta_description?: string | null;
  canonical_url?: string | null;
  social_image_key?: string | null;
  tags?: string[] | null;
  court_case_id?: string | null;
  court_name?: string | null;
  citation?: string | null;
  status?: 'draft' | 'in_review' | 'scheduled' | 'published' | 'archived';
  version?: number;
  word_count?: number;
  reading_time_minutes?: number;
  scheduled_at?: string | null;
  pdf_storage_key?: string | null;
  pdf_size_bytes?: number | null;
  pdf_checksum?: string | null;
  deleted_at?: string | null;
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
  const [deletedAt, setDeletedAt] = useState<string | null>(initialData?.deleted_at || null);
  const [scheduledAt, setScheduledAt] = useState<string | null>(initialData?.scheduled_at || null);

  // Form Fields
  const [title, setTitle] = useState(initialData?.title || '');
  const [subtitle, setSubtitle] = useState(initialData?.subtitle || '');
  const [summary, setSummary] = useState(initialData?.summary || '');
  const [bodyBlocks, setBodyBlocks] = useState<any[]>(() => {
    if (Array.isArray(initialData?.body_blocks)) {
      return initialData.body_blocks;
    }
    if (initialData?.body_blocks && Array.isArray((initialData.body_blocks as any).content)) {
      return (initialData.body_blocks as any).content;
    }
    return [];
  });
  const [articleType, setArticleType] = useState<ArticleType>(initialData?.article_type || 'news');
  const [accessLevel, setAccessLevel] = useState<'free' | 'premium'>(
    initialData?.access_level || 'free'
  );
  const [category, setCategory] = useState(initialData?.category || 'Supreme Court');
  const [sectionId, setSectionId] = useState(initialData?.section_id || '');
  const [tagsInput, setTagsInput] = useState((initialData?.tags || []).join(', '));
  const [courtCaseId, setCourtCaseId] = useState(initialData?.court_case_id || '');
  const [courtName, setCourtName] = useState(initialData?.court_name || 'Supreme Court of India');
  const [citation, setCitation] = useState(initialData?.citation || '');

  // SEO & Media fields
  const [metaTitle, setMetaTitle] = useState(initialData?.meta_title || '');
  const [metaDescription, setMetaDescription] = useState(initialData?.meta_description || '');
  const [canonicalUrl, setCanonicalUrl] = useState(initialData?.canonical_url || '');
  const [socialImageKey, setSocialImageKey] = useState(initialData?.social_image_key || '');
  const [featuredImageKey, setFeaturedImageKey] = useState(initialData?.featured_image_key || '');
  const [featuredImageAlt, setFeaturedImageAlt] = useState(initialData?.featured_image_alt || '');
  const [featuredImageCaption, setFeaturedImageCaption] = useState(
    initialData?.featured_image_caption || ''
  );

  // Sections and Cases lists
  const [sections, setSections] = useState<any[]>([]);
  const [liveCases, setLiveCases] = useState<any[]>([]);

  // Attachments
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

  // State flags
  const [saving, setSaving] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [conflictModalOpen, setConflictModalOpen] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(
    null
  );

  // Scheduling Modal
  const [scheduleModalOpen, setScheduleModalOpen] = useState(false);
  const [scheduleDateInput, setScheduleDateInput] = useState('');

  // Revisions Drawer & Diff
  const [revisionsDrawerOpen, setRevisionsDrawerOpen] = useState(false);
  const [revisions, setRevisions] = useState<any[]>([]);
  const [revisionsLoading, setRevisionsLoading] = useState(false);
  const [selectedRevision, setSelectedRevision] = useState<any | null>(null);
  const [restoringRevision, setRestoringRevision] = useState(false);

  // Send Back Modal
  const [sendBackOpen, setSendBackOpen] = useState(false);
  const [sendBackComment, setSendBackComment] = useState('Needs minor factual corrections.');

  // Accordion state
  const [seoOpen, setSeoOpen] = useState(false);

  // Live calculation of word count and reading time
  const metrics = useMemo(() => {
    let text = (summary || '') + ' ';
    const extractText = (node: any) => {
      if (!node) return;
      if (typeof node === 'string') {
        text += node + ' ';
        return;
      }
      if (node.text) text += node.text + ' ';
      if (Array.isArray(node.content)) {
        node.content.forEach(extractText);
      }
    };
    const nodes = Array.isArray(bodyBlocks)
      ? bodyBlocks
      : bodyBlocks && Array.isArray((bodyBlocks as any).content)
      ? (bodyBlocks as any).content
      : [];
    nodes.forEach(extractText);
    const words = text.trim().split(/\s+/).filter(Boolean).length;
    const readingTime = Math.max(1, Math.ceil(words / 200));
    return { words, readingTime };
  }, [summary, bodyBlocks]);

  useEffect(() => {
    // Fetch live cases & taxonomy sections
    const fetchData = async () => {
      try {
        const [casesRes, sectionsRes] = await Promise.all([
          apiClient.GET('/v1/cases/live'),
          apiClient.GET('/v1/cms/sections'),
        ]);
        if (casesRes.data) {
          const items = (casesRes.data as any)?.cases || (Array.isArray(casesRes.data) ? casesRes.data : []);
          setLiveCases(Array.isArray(items) ? items : []);
        }
        if (sectionsRes.data) {
          const secList = (sectionsRes.data as any)?.sections || (Array.isArray(sectionsRes.data) ? sectionsRes.data : []);
          setSections(Array.isArray(secList) ? secList : []);
        }
      } catch {
        // Ignore background load failures
      }
    };
    fetchData();
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
            section_id: sectionId.trim() ? sectionId.trim() : undefined,
            featured_image_key: featuredImageKey.trim() ? featuredImageKey.trim() : undefined,
            featured_image_alt: featuredImageAlt.trim() ? featuredImageAlt.trim() : undefined,
            featured_image_caption: featuredImageCaption.trim() ? featuredImageCaption.trim() : undefined,
            meta_title: metaTitle.trim() ? metaTitle.trim() : undefined,
            meta_description: metaDescription.trim() ? metaDescription.trim() : undefined,
            canonical_url: canonicalUrl.trim() ? canonicalUrl.trim() : undefined,
            social_image_key: socialImageKey.trim() ? socialImageKey.trim() : undefined,
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
            section_id: sectionId.trim() ? sectionId.trim() : undefined,
            featured_image_key: featuredImageKey.trim() ? featuredImageKey.trim() : undefined,
            featured_image_alt: featuredImageAlt.trim() ? featuredImageAlt.trim() : undefined,
            featured_image_caption: featuredImageCaption.trim() ? featuredImageCaption.trim() : undefined,
            meta_title: metaTitle.trim() ? metaTitle.trim() : undefined,
            meta_description: metaDescription.trim() ? metaDescription.trim() : undefined,
            canonical_url: canonicalUrl.trim() ? canonicalUrl.trim() : undefined,
            social_image_key: socialImageKey.trim() ? socialImageKey.trim() : undefined,
            tags: tags.length > 0 ? tags : undefined,
            court_case_id: courtCaseId.trim() ? courtCaseId.trim() : undefined,
            court_name: courtName.trim() ? courtName.trim() : undefined,
            citation: citation.trim() ? citation.trim() : undefined,
          },
        });

        if (response.status === 409) {
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
        setScheduledAt(null);
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

  // Schedule Article
  const handleScheduleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!articleId || !scheduleDateInput) return;

    setActionLoading(true);
    try {
      const scheduledIso = new Date(scheduleDateInput).toISOString();
      const { data, error, response } = await apiClient.POST('/v1/cms/articles/{id}/schedule', {
        params: { path: { id: articleId } },
        body: { scheduled_at: scheduledIso },
      });

      if (response.ok && data) {
        setStatus(data.status);
        setScheduledAt(data.scheduled_at || scheduledIso);
        setScheduleModalOpen(false);
        setStatusMessage({
          text: `Article scheduled for publication at ${new Date(scheduledIso).toLocaleString()}!`,
          type: 'success',
        });
      } else {
        alert((error as any)?.message || 'Failed to schedule article publication.');
      }
    } catch (err: any) {
      alert(err.message || 'Error scheduling publication.');
    } finally {
      setActionLoading(false);
    }
  };

  // Soft Delete Article
  const handleSoftDelete = async () => {
    if (!articleId) return;
    if (!confirm('Are you sure you want to move this article to trash? It can be restored later.')) {
      return;
    }
    setActionLoading(true);
    try {
      const { response, error } = await apiClient.DELETE('/v1/cms/articles/{id}', {
        params: { path: { id: articleId } },
      });
      if (response.ok) {
        setDeletedAt(new Date().toISOString());
        setStatusMessage({ text: 'Article moved to trash (soft-deleted).', type: 'success' });
      } else {
        alert((error as any)?.message || 'Failed to soft delete article');
      }
    } catch (err: any) {
      alert(err.message || 'Network error');
    } finally {
      setActionLoading(false);
    }
  };

  // Restore Soft-Deleted Article
  const handleRestoreDeleted = async () => {
    if (!articleId) return;
    setActionLoading(true);
    try {
      const { response, error } = await apiClient.POST('/v1/cms/articles/{id}/restore', {
        params: { path: { id: articleId } },
      });
      if (response.ok) {
        setDeletedAt(null);
        setStatusMessage({ text: 'Article restored back from trash.', type: 'success' });
      } else {
        alert((error as any)?.message || 'Failed to restore article');
      }
    } catch (err: any) {
      alert(err.message || 'Network error');
    } finally {
      setActionLoading(false);
    }
  };

  // Open Revisions Drawer
  const handleOpenRevisions = async () => {
    if (!articleId) return;
    setRevisionsDrawerOpen(true);
    setRevisionsLoading(true);
    setSelectedRevision(null);
    try {
      const { data, response } = await apiClient.GET('/v1/cms/articles/{id}/revisions', {
        params: { path: { id: articleId } },
      });
      if (response.ok && data) {
        setRevisions((data as any).revisions || []);
      }
    } catch {
      // Ignore
    } finally {
      setRevisionsLoading(false);
    }
  };

  // Select Revision for Diff
  const handleSelectRevision = async (revVersion: number) => {
    if (!articleId) return;
    try {
      const { data, response } = await apiClient.GET('/v1/cms/articles/{id}/revisions/{version}', {
        params: { path: { id: articleId, version: revVersion } },
      });
      if (response.ok && data) {
        setSelectedRevision(data);
      }
    } catch {
      // Ignore
    }
  };

  // Restore Draft to Revision
  const handleRestoreRevision = async (revVersion: number) => {
    if (!articleId) return;
    if (!confirm(`Restore draft to Revision v${revVersion}? Current unsaved edits will be replaced.`)) {
      return;
    }
    setRestoringRevision(true);
    try {
      const { data, error, response } = await apiClient.POST(
        '/v1/cms/articles/{id}/revisions/{version}/restore',
        {
          params: { path: { id: articleId, version: revVersion } },
        }
      );
      if (response.ok && data) {
        setTitle(data.title);
        setSubtitle(data.subtitle || '');
        setSummary(data.summary);
        setBodyBlocks(data.body_blocks || []);
        setVersion(data.version);
        setStatus(data.status);
        setRevisionsDrawerOpen(false);
        setStatusMessage({
          text: `Draft successfully restored to Revision v${revVersion}! New version is v${data.version}.`,
          type: 'success',
        });
      } else {
        alert((error as any)?.message || 'Failed to restore revision');
      }
    } catch (err: any) {
      alert(err.message || 'Network error');
    } finally {
      setRestoringRevision(false);
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
      {/* Deleted Banner */}
      {deletedAt && (
        <div className="bg-red-500/10 border border-red-500/20 p-4 rounded-2xl flex items-center justify-between text-red-400">
          <div className="flex items-center gap-3">
            <Trash2 className="h-5 w-5 shrink-0" />
            <div>
              <p className="font-bold text-sm">This article is currently in the Trash</p>
              <p className="text-xs text-red-300">Soft-deleted at {new Date(deletedAt).toLocaleString()}</p>
            </div>
          </div>
          {hasRole('editor', 'admin') && (
            <button
              onClick={handleRestoreDeleted}
              disabled={actionLoading}
              className="px-4 py-1.5 bg-red-600 hover:bg-red-500 text-white font-bold text-xs rounded-xl shadow transition"
            >
              Restore Article
            </button>
          )}
        </div>
      )}

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
              <span
                className={`text-xs uppercase tracking-wider font-extrabold px-2.5 py-0.5 rounded-full border ${
                  status === 'published'
                    ? 'bg-emerald-50 text-emerald-800 border-emerald-200'
                    : status === 'scheduled'
                    ? 'bg-purple-50 text-purple-800 border-purple-200'
                    : status === 'in_review'
                    ? 'bg-blue-50 text-blue-800 border-blue-200'
                    : 'bg-slate-100 text-slate-800 border-slate-200'
                }`}
              >
                {status.replace('_', ' ')}
              </span>
              {articleId && (
                <span className="font-mono text-xs font-semibold text-slate-400 bg-slate-50 px-2 py-0.5 rounded border border-slate-200">
                  v{version}
                </span>
              )}
              {scheduledAt && (
                <span className="text-[11px] font-medium text-purple-600 bg-purple-50 border border-purple-100 px-2 py-0.5 rounded-md flex items-center gap-1">
                  <Clock className="h-3 w-3" />
                  Scheduled: {new Date(scheduledAt).toLocaleString()}
                </span>
              )}
            </div>
            <p className="text-[11px] text-slate-400 mt-0.5 flex items-center gap-2">
              <span>{isNew ? 'New Editorial Draft' : `Editing Article #${articleId?.substring(0, 8)}...`}</span>
              <span>•</span>
              <span className="text-slate-600 font-medium">
                {metrics.words.toLocaleString()} words (~{metrics.readingTime} min read)
              </span>
            </p>
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex items-center gap-2 flex-wrap">
          {/* Revisions History Drawer Trigger */}
          {articleId && (
            <button
              type="button"
              onClick={handleOpenRevisions}
              className="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 px-3 py-2 text-xs font-bold text-slate-700 transition"
              title="View revision snapshots and side-by-side diff"
            >
              <History className="h-4 w-4 text-slate-500" />
              Revisions
            </button>
          )}

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

          {/* Schedule Publication */}
          {articleId && status !== 'published' && hasRole('editor', 'admin') && (
            <button
              type="button"
              onClick={() => {
                const now = new Date();
                now.setHours(now.getHours() + 1);
                setScheduleDateInput(now.toISOString().slice(0, 16));
                setScheduleModalOpen(true);
              }}
              disabled={actionLoading || saving}
              className="inline-flex items-center gap-1.5 rounded-xl border border-purple-300 bg-purple-50 hover:bg-purple-100 px-3.5 py-2 text-xs font-bold text-purple-900 transition"
            >
              <Calendar className="h-4 w-4 text-purple-600" />
              Schedule...
            </button>
          )}

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

          {/* Editor/Admin Actions when in_review or scheduled */}
          {(status === 'in_review' || status === 'scheduled') && hasRole('editor', 'admin') && (
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

          {/* Soft Delete */}
          {articleId && !deletedAt && hasRole('editor', 'admin') && (
            <button
              type="button"
              onClick={handleSoftDelete}
              disabled={actionLoading || saving}
              className="p-2 text-slate-400 hover:text-red-600 hover:bg-red-50 rounded-xl transition"
              title="Move to Trash"
            >
              <Trash2 className="h-4 w-4" />
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
          <button type="button" onClick={() => setStatusMessage(null)} className="underline ml-4">
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
                placeholder="Brief 2-3 sentence overview of the judgment, order, or legal news report..."
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-3 text-xs text-slate-800 placeholder:text-slate-400 focus:border-amber-400 focus:outline-none focus:ring-1 focus:ring-amber-400"
              />
            </div>
          </div>

          {/* TipTap Rich Text Editor */}
          <div className="rounded-2xl bg-white border border-slate-200 p-5 shadow-sm space-y-3">
            <div className="flex items-center justify-between">
              <label className="block text-xs font-bold uppercase tracking-wider text-slate-700">
                Article Body Content
              </label>
              <div className="flex items-center gap-3 text-xs text-slate-500 font-mono">
                <span>{metrics.words.toLocaleString()} words</span>
                <span>•</span>
                <span>~{metrics.readingTime} min read</span>
              </div>
            </div>
            <TipTapEditor content={bodyBlocks} onChange={(blocks) => setBodyBlocks(blocks)} />
          </div>

          {/* SEO & Social Metadata Drawer/Accordion */}
          <div className="rounded-2xl bg-white border border-slate-200 shadow-sm overflow-hidden">
            <button
              type="button"
              onClick={() => setSeoOpen(!seoOpen)}
              className="w-full px-5 py-4 flex items-center justify-between text-left hover:bg-slate-50 transition"
            >
              <div className="flex items-center gap-2">
                <Globe className="h-4 w-4 text-amber-500" />
                <span className="text-xs font-extrabold uppercase tracking-wider text-slate-800">
                  Search Engine Optimization (SEO) &amp; Social Meta
                </span>
              </div>
              {seoOpen ? <ChevronUp className="h-4 w-4 text-slate-400" /> : <ChevronDown className="h-4 w-4 text-slate-400" />}
            </button>

            {seoOpen && (
              <div className="p-5 border-t border-slate-200 bg-slate-50/40 space-y-4">
                <div>
                  <div className="flex justify-between items-center mb-1">
                    <label className="text-xs font-bold text-slate-700">Meta Title</label>
                    <span className="text-[10px] text-slate-400 font-mono">{metaTitle.length}/60 chars</span>
                  </div>
                  <input
                    type="text"
                    value={metaTitle}
                    onChange={(e) => setMetaTitle(e.target.value)}
                    placeholder="Custom meta title for Google SERPs (defaults to Headline)"
                    className="w-full rounded-xl border border-slate-200 bg-white p-2.5 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                  />
                </div>

                <div>
                  <div className="flex justify-between items-center mb-1">
                    <label className="text-xs font-bold text-slate-700">Meta Description</label>
                    <span className="text-[10px] text-slate-400 font-mono">{metaDescription.length}/160 chars</span>
                  </div>
                  <textarea
                    rows={2}
                    value={metaDescription}
                    onChange={(e) => setMetaDescription(e.target.value)}
                    placeholder="Search snippet description for Google and Twitter cards (defaults to Summary)"
                    className="w-full rounded-xl border border-slate-200 bg-white p-2.5 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                  />
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">Canonical URL</label>
                    <input
                      type="url"
                      value={canonicalUrl}
                      onChange={(e) => setCanonicalUrl(e.target.value)}
                      placeholder="https://clientdomain.com/sc-privacy-ruling"
                      className="w-full rounded-xl border border-slate-200 bg-white p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">Social Share Image Key</label>
                    <input
                      type="text"
                      value={socialImageKey}
                      onChange={(e) => setSocialImageKey(e.target.value)}
                      placeholder="articles/social/privacy-card.jpg"
                      className="w-full rounded-xl border border-slate-200 bg-white p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none font-mono"
                    />
                  </div>
                </div>

                <div className="border-t border-slate-200 pt-4 space-y-3">
                  <h5 className="text-xs font-extrabold uppercase tracking-wider text-slate-700 flex items-center gap-1.5">
                    <ImageIcon className="h-3.5 w-3.5 text-slate-500" />
                    Featured Hero Image
                  </h5>
                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                    <div className="sm:col-span-1">
                      <label className="block text-[11px] font-bold text-slate-600 mb-1">Storage Key</label>
                      <input
                        type="text"
                        value={featuredImageKey}
                        onChange={(e) => setFeaturedImageKey(e.target.value)}
                        placeholder="images/sc-building.jpg"
                        className="w-full rounded-xl border border-slate-200 bg-white p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none font-mono"
                      />
                    </div>
                    <div className="sm:col-span-1">
                      <label className="block text-[11px] font-bold text-slate-600 mb-1">Alt Text</label>
                      <input
                        type="text"
                        value={featuredImageAlt}
                        onChange={(e) => setFeaturedImageAlt(e.target.value)}
                        placeholder="Supreme Court of India facade"
                        className="w-full rounded-xl border border-slate-200 bg-white p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                      />
                    </div>
                    <div className="sm:col-span-1">
                      <label className="block text-[11px] font-bold text-slate-600 mb-1">Caption / Credit</label>
                      <input
                        type="text"
                        value={featuredImageCaption}
                        onChange={(e) => setFeaturedImageCaption(e.target.value)}
                        placeholder="Photo: Express Archives"
                        className="w-full rounded-xl border border-slate-200 bg-white p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                      />
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Judgment Attachment Section */}
          <div className="rounded-2xl bg-white border border-slate-200 p-5 shadow-sm space-y-4">
            <h4 className="text-xs font-extrabold uppercase tracking-wider text-slate-700 flex items-center gap-1.5">
              <FileText className="h-4 w-4 text-slate-600" />
              Attached Judgment / Order PDF
            </h4>
            <PDFUploader
              articleId={articleId}
              currentAttachment={attachment}
              onUploadSuccess={handleUploadSuccess}
            />
          </div>
        </div>

        {/* RIGHT COLUMN: Sidebar Controls */}
        <div className="lg:col-span-4 space-y-6">
          {/* Access Tier */}
          <div className="rounded-2xl bg-white border border-slate-200 p-5 shadow-sm space-y-3">
            <h4 className="text-xs font-extrabold uppercase tracking-wider text-slate-700 flex items-center gap-1.5">
              <ShieldCheck className="h-4 w-4 text-slate-600" />
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

          {/* Section & Taxonomy Classification */}
          <div className="rounded-2xl bg-white border border-slate-200 p-5 shadow-sm space-y-4">
            <h4 className="text-xs font-extrabold uppercase tracking-wider text-slate-700 flex items-center gap-1.5">
              <FolderTree className="h-4 w-4 text-slate-600" />
              Taxonomy &amp; Section
            </h4>

            <div>
              <label className="block text-xs font-bold text-slate-700 mb-1">
                Primary Section *
              </label>
              <select
                value={sectionId}
                onChange={(e) => setSectionId(e.target.value)}
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-semibold text-slate-800 focus:border-amber-400 focus:outline-none"
              >
                <option value="">-- No specific section (General) --</option>
                {sections.map((sec) => (
                  <option key={sec.id} value={sec.id}>
                    {sec.name}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-xs font-bold text-slate-700 mb-1">
                Article Format / Type
              </label>
              <select
                value={articleType}
                onChange={(e) => setArticleType(e.target.value as any)}
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-semibold text-slate-800 focus:border-amber-400 focus:outline-none capitalize"
              >
                <option value="news">News Report</option>
                <option value="top_story">Top Story</option>
                <option value="column">Column</option>
                <option value="opinion">Opinion</option>
                <option value="interview">Interview</option>
                <option value="round_up">Round-up</option>
                <option value="digest">Digest</option>
                <option value="know_the_law">Know The Law</option>
                <option value="book_review">Book Review</option>
                <option value="job_update">Job Update</option>
                <option value="event">Event</option>
                <option value="video">Video</option>
                <option value="cartoon">Cartoon</option>
                <option value="commentary">Legal Commentary</option>
                <option value="editorial">Editorial</option>
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
                <option value="Taxation">Taxation</option>
                <option value="Arbitration">Arbitration</option>
                <option value="Human Rights">Human Rights</option>
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

      {/* Schedule Modal */}
      {scheduleModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 backdrop-blur-sm p-4">
          <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl border border-slate-200">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-base font-extrabold text-slate-900 flex items-center gap-2">
                <Calendar className="h-5 w-5 text-purple-600" />
                Schedule Publication
              </h3>
              <button
                type="button"
                onClick={() => setScheduleModalOpen(false)}
                className="p-1 rounded-lg text-slate-400 hover:text-slate-600"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <p className="text-xs text-slate-500 mb-4 leading-relaxed">
              Choose the exact future date and time for automated release. The background publisher worker checks scheduled articles every 10 seconds.
            </p>

            <form onSubmit={handleScheduleSubmit} className="space-y-4">
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-slate-700 mb-1">
                  Target Date &amp; Time
                </label>
                <input
                  type="datetime-local"
                  required
                  value={scheduleDateInput}
                  onChange={(e) => setScheduleDateInput(e.target.value)}
                  className="w-full rounded-xl border border-slate-300 p-2.5 text-xs text-slate-800 font-mono focus:border-purple-500 focus:outline-none"
                />
              </div>

              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setScheduleModalOpen(false)}
                  className="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={actionLoading}
                  className="rounded-xl bg-purple-600 hover:bg-purple-700 px-4 py-2 text-xs font-bold text-white shadow transition disabled:opacity-50"
                >
                  {actionLoading ? 'Scheduling...' : 'Set Scheduled Time'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Revisions History Drawer */}
      {revisionsDrawerOpen && (
        <div className="fixed inset-0 z-50 flex justify-end bg-slate-950/60 backdrop-blur-sm">
          <div className="w-full max-w-2xl bg-white h-full shadow-2xl flex flex-col animate-in slide-in-from-right duration-200">
            {/* Drawer Header */}
            <div className="flex items-center justify-between p-5 border-b border-slate-200 bg-slate-50">
              <div className="flex items-center gap-2">
                <History className="h-5 w-5 text-slate-700" />
                <h3 className="font-extrabold text-base text-slate-900">Revision History Snapshots</h3>
              </div>
              <button
                type="button"
                onClick={() => setRevisionsDrawerOpen(false)}
                className="p-1.5 rounded-lg text-slate-400 hover:text-slate-700 hover:bg-slate-200 transition"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            {/* Drawer Content */}
            <div className="flex-1 overflow-y-auto p-5 space-y-4">
              {revisionsLoading ? (
                <div className="py-20 flex flex-col items-center justify-center text-slate-400 gap-2">
                  <Loader2 className="h-6 w-6 animate-spin text-slate-600" />
                  <p className="text-xs">Loading revisions...</p>
                </div>
              ) : revisions.length === 0 ? (
                <div className="text-center py-16 text-slate-400 text-xs">
                  No historical revisions recorded yet. Revisions are created automatically whenever edits are saved.
                </div>
              ) : (
                <div className="space-y-4">
                  {revisions.map((rev) => (
                    <div
                      key={rev.id}
                      className={`border rounded-2xl p-4 transition ${
                        selectedRevision?.version === rev.version
                          ? 'border-amber-500 bg-amber-50/20'
                          : 'border-slate-200 bg-white hover:border-slate-300'
                      }`}
                    >
                      <div className="flex items-center justify-between mb-2">
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-xs font-bold px-2 py-0.5 rounded bg-slate-100 text-slate-800">
                            v{rev.version}
                          </span>
                          <span className="text-xs text-slate-400">
                            {new Date(rev.created_at).toLocaleString()}
                          </span>
                        </div>
                        <div className="flex items-center gap-2">
                          <button
                            type="button"
                            onClick={() => handleSelectRevision(rev.version)}
                            className="px-2.5 py-1 text-xs font-semibold text-slate-600 hover:text-slate-900 bg-slate-100 rounded-lg transition"
                          >
                            {selectedRevision?.version === rev.version ? 'Inspecting' : 'Inspect'}
                          </button>
                          {hasRole('editor', 'admin') && (
                            <button
                              type="button"
                              onClick={() => handleRestoreRevision(rev.version)}
                              disabled={restoringRevision}
                              className="px-2.5 py-1 text-xs font-bold text-amber-900 bg-amber-400 hover:bg-amber-300 rounded-lg shadow-sm transition"
                            >
                              Restore
                            </button>
                          )}
                        </div>
                      </div>
                      <p className="text-xs font-bold text-slate-800 line-clamp-1">{rev.title}</p>
                      <p className="text-[11px] text-slate-500 line-clamp-1 mt-0.5">{rev.summary}</p>
                    </div>
                  ))}

                  {/* Diff Inspector */}
                  {selectedRevision && (
                    <div className="mt-6 border-t border-slate-200 pt-5">
                      <h4 className="font-bold text-xs uppercase tracking-wider text-slate-700 mb-3 flex items-center justify-between">
                        <span>Side-by-Side Comparison: v{selectedRevision.version} vs Current (v{version})</span>
                        <button
                          onClick={() => setSelectedRevision(null)}
                          className="text-[11px] text-slate-400 hover:underline"
                        >
                          Close Diff
                        </button>
                      </h4>
                      <div className="grid grid-cols-2 gap-4 text-xs font-mono">
                        <div className="p-3 bg-red-50/60 border border-red-100 rounded-xl space-y-2">
                          <div className="font-bold text-red-900 uppercase text-[10px]">
                            Revision v{selectedRevision.version}
                          </div>
                          <div>
                            <span className="text-red-700 font-bold block text-[10px]">Title:</span>
                            <p className="text-slate-800">{selectedRevision.title}</p>
                          </div>
                          <div>
                            <span className="text-red-700 font-bold block text-[10px]">Summary:</span>
                            <p className="text-slate-800">{selectedRevision.summary}</p>
                          </div>
                        </div>
                        <div className="p-3 bg-emerald-50/60 border border-emerald-100 rounded-xl space-y-2">
                          <div className="font-bold text-emerald-900 uppercase text-[10px]">
                            Current Working Copy (v{version})
                          </div>
                          <div>
                            <span className="text-emerald-700 font-bold block text-[10px]">Title:</span>
                            <p className="text-slate-800">{title}</p>
                          </div>
                          <div>
                            <span className="text-emerald-700 font-bold block text-[10px]">Summary:</span>
                            <p className="text-slate-800">{summary}</p>
                          </div>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

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
            <h3 className="text-base font-extrabold text-slate-900 mb-2">Send Back to Reporter</h3>
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
