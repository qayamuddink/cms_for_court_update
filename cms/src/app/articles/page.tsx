'use client';

import React, { useState, useEffect, Suspense } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import apiClient from '@/lib/api/client';
import { useAuth } from '@/lib/auth/context';
import ProtectedRoute from '@/components/auth/ProtectedRoute';
import CMSLayout from '@/components/layout/CMSLayout';
import {
  FileText,
  Plus,
  Search,
  Filter,
  CheckCircle2,
  Clock,
  Archive,
  Send,
  RotateCcw,
  ExternalLink,
  Edit3,
  Loader2,
  Paperclip,
  Sparkles,
  ShieldAlert,
} from 'lucide-react';

interface ArticleItem {
  id: string;
  slug: string;
  title: string;
  subtitle?: string | null;
  summary: string;
  article_type: 'news' | 'commentary' | 'editorial' | 'judgment_summary';
  access_level: 'free' | 'premium';
  status: 'draft' | 'in_review' | 'published' | 'archived';
  category: string;
  author_id: string;
  version: number;
  pdf_storage_key?: string | null;
  created_at: string;
  updated_at: string;
  published_at?: string | null;
}

function ArticlesContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const initialStatus = searchParams.get('status') || '';
  const { user, hasRole } = useAuth();

  const [articles, setArticles] = useState<ArticleItem[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [activeStatus, setActiveStatus] = useState<string>(initialStatus);
  const [searchQuery, setSearchQuery] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('');

  // Send Back modal state
  const [sendBackArticle, setSendBackArticle] = useState<ArticleItem | null>(null);
  const [sendBackComment, setSendBackComment] = useState('Needs minor factual corrections.');
  const [actionLoading, setActionLoading] = useState(false);
  const [actionMessage, setActionMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null);

  const fetchArticles = async () => {
    setLoading(true);
    try {
      const queryParams: Record<string, string> = { limit: '50' };
      if (activeStatus) queryParams.status = activeStatus;
      if (categoryFilter) queryParams.category = categoryFilter;
      if (searchQuery.trim()) queryParams.search = searchQuery.trim();

      const { data, response } = await apiClient.GET('/v1/cms/articles', {
        params: { query: queryParams as any },
      });

      if (response.ok && data) {
        setArticles(data.articles as ArticleItem[]);
        setTotal(data.total);
      }
    } catch {
      // Ignore background error
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchArticles();
  }, [activeStatus, categoryFilter]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    fetchArticles();
  };

  // Submit for Review
  const handleSubmitForReview = async (articleId: string) => {
    setActionLoading(true);
    setActionMessage(null);
    try {
      const { response, error } = await apiClient.POST('/v1/cms/articles/{id}/submit', {
        params: { path: { id: articleId } },
      });
      if (response.ok) {
        setActionMessage({ text: 'Article submitted to editorial review queue.', type: 'success' });
        fetchArticles();
      } else {
        setActionMessage({ text: (error as any)?.message || 'Failed to submit article', type: 'error' });
      }
    } catch (err: any) {
      setActionMessage({ text: err.message || 'Network error', type: 'error' });
    } finally {
      setActionLoading(false);
    }
  };

  // Publish Article
  const handlePublish = async (articleId: string) => {
    setActionLoading(true);
    setActionMessage(null);
    try {
      const { response, error } = await apiClient.POST('/v1/cms/articles/{id}/publish', {
        params: { path: { id: articleId } },
      });
      if (response.ok) {
        setActionMessage({ text: 'Article published live successfully!', type: 'success' });
        fetchArticles();
      } else {
        setActionMessage({ text: (error as any)?.message || 'Failed to publish article', type: 'error' });
      }
    } catch (err: any) {
      setActionMessage({ text: err.message || 'Network error', type: 'error' });
    } finally {
      setActionLoading(false);
    }
  };

  // Archive Article
  const handleArchive = async (articleId: string) => {
    setActionLoading(true);
    setActionMessage(null);
    try {
      const { response, error } = await apiClient.POST('/v1/cms/articles/{id}/archive', {
        params: { path: { id: articleId } },
      });
      if (response.ok) {
        setActionMessage({ text: 'Article moved to archive.', type: 'success' });
        fetchArticles();
      } else {
        setActionMessage({ text: (error as any)?.message || 'Failed to archive article', type: 'error' });
      }
    } catch (err: any) {
      setActionMessage({ text: err.message || 'Network error', type: 'error' });
    } finally {
      setActionLoading(false);
    }
  };

  // Confirm Send Back
  const handleConfirmSendBack = async () => {
    if (!sendBackArticle) return;
    setActionLoading(true);
    try {
      const { response, error } = await apiClient.POST('/v1/cms/articles/{id}/send-back', {
        params: { path: { id: sendBackArticle.id } },
        body: { comment: sendBackComment.trim() || 'Please revise and resubmit.' },
      });
      if (response.ok) {
        setSendBackArticle(null);
        setActionMessage({ text: 'Article returned to reporter draft.', type: 'success' });
        fetchArticles();
      } else {
        alert((error as any)?.message || 'Failed to send back article');
      }
    } catch (err: any) {
      alert(err.message || 'Network error');
    } finally {
      setActionLoading(false);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'draft':
        return 'bg-slate-100 text-slate-700 border-slate-200';
      case 'in_review':
        return 'bg-blue-100 text-blue-800 border-blue-200';
      case 'published':
        return 'bg-emerald-100 text-emerald-800 border-emerald-200';
      case 'archived':
        return 'bg-amber-100 text-amber-800 border-amber-200';
      default:
        return 'bg-slate-100 text-slate-800 border-slate-200';
    }
  };

  return (
    <CMSLayout title="Editorial Articles & Reviews" badge={`${total} Total`}>
      {/* Action Banner Notifications */}
      {actionMessage && (
        <div
          className={`mb-6 flex items-center justify-between rounded-xl border p-4 text-sm font-semibold transition ${
            actionMessage.type === 'success'
              ? 'bg-emerald-50 border-emerald-200 text-emerald-800'
              : 'bg-red-50 border-red-200 text-red-800'
          }`}
        >
          <span>{actionMessage.text}</span>
          <button
            type="button"
            onClick={() => setActionMessage(null)}
            className="text-xs font-bold underline ml-4"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Top Header Controls: New Draft & Search */}
      <div className="flex flex-col md:flex-row items-stretch md:items-center justify-between gap-4 mb-6">
        {/* Status Tabs */}
        <div className="flex items-center gap-1 overflow-x-auto pb-1 sm:pb-0 scrollbar-none">
          {[
            { label: 'All Articles', value: '' },
            { label: 'Working Drafts', value: 'draft' },
            { label: 'In Review Queue', value: 'in_review' },
            { label: 'Published Live', value: 'published' },
            { label: 'Archived', value: 'archived' },
          ].map((tab) => (
            <button
              key={tab.value}
              type="button"
              onClick={() => setActiveStatus(tab.value)}
              className={`rounded-xl px-4 py-2 text-xs font-bold transition whitespace-nowrap ${
                activeStatus === tab.value
                  ? 'bg-slate-900 text-white shadow-sm'
                  : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* New Draft Button */}
        <a
          href="/articles/new"
          className="inline-flex items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 px-5 py-2.5 text-xs font-extrabold text-slate-950 shadow-md shadow-amber-500/20 hover:from-amber-400 hover:to-amber-500 transition"
        >
          <Plus className="h-4 w-4 stroke-[2.5]" />
          Compose New Article
        </a>
      </div>

      {/* Search & Category Filter Bar */}
      <div className="rounded-2xl bg-white border border-slate-200 p-4 shadow-sm mb-6 flex flex-col sm:flex-row items-center gap-3">
        <form onSubmit={handleSearch} className="relative flex-1 w-full">
          <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400" />
          <input
            type="text"
            placeholder="Search articles by title, keywords, or summary..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full rounded-xl border border-slate-200 bg-slate-50/50 pl-10 pr-4 py-2 text-xs text-slate-800 placeholder:text-slate-400 focus:border-amber-400 focus:outline-none focus:ring-1 focus:ring-amber-400"
          />
        </form>

        <div className="flex items-center gap-2 w-full sm:w-auto">
          <select
            value={categoryFilter}
            onChange={(e) => setCategoryFilter(e.target.value)}
            className="w-full sm:w-48 rounded-xl border border-slate-200 bg-slate-50/50 px-3 py-2 text-xs font-semibold text-slate-700 focus:border-amber-400 focus:outline-none"
          >
            <option value="">All Categories</option>
            <option value="Supreme Court">Supreme Court</option>
            <option value="High Courts">High Courts</option>
            <option value="Constitutional Law">Constitutional Law</option>
            <option value="Criminal Law">Criminal Law</option>
            <option value="Corporate Law">Corporate Law</option>
            <option value="Human Rights">Human Rights</option>
          </select>

          <button
            type="button"
            onClick={fetchArticles}
            className="rounded-xl border border-slate-200 bg-slate-100 hover:bg-slate-200 p-2 text-slate-600 transition"
            title="Refresh list"
          >
            <RotateCcw className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Articles Stream / Table */}
      <div className="rounded-2xl bg-white border border-slate-200 shadow-md overflow-hidden">
        {loading ? (
          <div className="py-24 text-center text-slate-400">
            <Loader2 className="h-8 w-8 animate-spin mx-auto mb-3 text-amber-500" />
            <p className="text-xs font-semibold">Loading editorial articles...</p>
          </div>
        ) : articles.length === 0 ? (
          <div className="py-24 text-center text-slate-400">
            <FileText className="h-12 w-12 text-slate-300 mx-auto mb-3" />
            <p className="text-base font-bold text-slate-700">No articles found</p>
            <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
              {activeStatus ? `No articles matching status "${activeStatus}".` : 'No article drafts created yet. Click "Compose New Article" to write one.'}
            </p>
          </div>
        ) : (
          <div className="divide-y divide-slate-100">
            {articles.map((article) => (
              <div
                key={article.id}
                className="p-5 hover:bg-slate-50/60 transition flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4"
              >
                {/* Left: Article Meta & Title */}
                <div className="space-y-1.5 flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span
                      className={`text-[10px] uppercase tracking-wider font-extrabold px-2 py-0.5 rounded border ${getStatusBadge(
                        article.status
                      )}`}
                    >
                      {article.status.replace('_', ' ')}
                    </span>
                    <span className="text-[10px] uppercase tracking-wider font-bold text-slate-600 bg-slate-100 px-2 py-0.5 rounded">
                      {article.category || 'General'}
                    </span>
                    <span className="text-[10px] uppercase tracking-wider font-semibold text-slate-500 bg-slate-50 px-2 py-0.5 rounded border border-slate-200">
                      {article.article_type.replace('_', ' ')}
                    </span>
                    {article.access_level === 'premium' ? (
                      <span className="text-[10px] uppercase tracking-wider font-bold bg-amber-100 text-amber-900 px-2 py-0.5 rounded border border-amber-300 flex items-center gap-1">
                        <Sparkles className="h-3 w-3 text-amber-700" />
                        Premium
                      </span>
                    ) : (
                      <span className="text-[10px] uppercase tracking-wider font-semibold text-slate-500">
                        Free
                      </span>
                    )}
                    <span className="font-mono text-[11px] text-slate-400 bg-slate-50 px-1.5 py-0.2 rounded border border-slate-200">
                      v{article.version}
                    </span>
                    {article.pdf_storage_key && (
                      <span className="inline-flex items-center gap-1 text-[11px] font-semibold text-blue-600 bg-blue-50 px-2 py-0.5 rounded border border-blue-200">
                        <Paperclip className="h-3 w-3" />
                        PDF Attached
                      </span>
                    )}
                  </div>

                  <h3 className="text-base font-extrabold text-slate-900 hover:text-amber-600 transition truncate">
                    <a href={`/articles/${article.id}/edit`}>{article.title}</a>
                  </h3>

                  {article.summary && (
                    <p className="text-xs text-slate-500 line-clamp-2 leading-relaxed">
                      {article.summary}
                    </p>
                  )}

                  <div className="flex items-center gap-4 text-[11px] text-slate-400 font-medium">
                    <span>
                      Updated {new Date(article.updated_at).toLocaleDateString('en-IN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                    </span>
                    {article.published_at && (
                      <span>
                        • Published {new Date(article.published_at).toLocaleDateString('en-IN', { month: 'short', day: 'numeric' })}
                      </span>
                    )}
                  </div>
                </div>

                {/* Right: Actions */}
                <div className="flex items-center gap-2 shrink-0 flex-wrap">
                  {/* Edit button */}
                  <a
                    href={`/articles/${article.id}/edit`}
                    className="inline-flex items-center gap-1.5 text-xs font-bold text-slate-700 bg-white border border-slate-200 hover:bg-slate-50 px-3 py-1.5 rounded-xl transition shadow-sm"
                  >
                    <Edit3 className="h-3.5 w-3.5" />
                    Edit
                  </a>

                  {/* Workflow buttons */}
                  {article.status === 'draft' && (
                    <button
                      type="button"
                      onClick={() => handleSubmitForReview(article.id)}
                      disabled={actionLoading}
                      className="inline-flex items-center gap-1.5 text-xs font-bold text-blue-700 bg-blue-50 hover:bg-blue-100 border border-blue-200 px-3 py-1.5 rounded-xl transition disabled:opacity-50"
                    >
                      <Send className="h-3.5 w-3.5" />
                      Submit for Review
                    </button>
                  )}

                  {article.status === 'in_review' && hasRole('editor', 'admin') && (
                    <>
                      <button
                        type="button"
                        onClick={() => handlePublish(article.id)}
                        disabled={actionLoading}
                        className="inline-flex items-center gap-1.5 text-xs font-bold text-slate-950 bg-emerald-400 hover:bg-emerald-500 px-3 py-1.5 rounded-xl transition shadow-sm disabled:opacity-50"
                      >
                        <CheckCircle2 className="h-3.5 w-3.5" />
                        Publish
                      </button>
                      <button
                        type="button"
                        onClick={() => setSendBackArticle(article)}
                        disabled={actionLoading}
                        className="inline-flex items-center gap-1.5 text-xs font-bold text-amber-800 bg-amber-50 hover:bg-amber-100 border border-amber-200 px-3 py-1.5 rounded-xl transition disabled:opacity-50"
                      >
                        <RotateCcw className="h-3.5 w-3.5" />
                        Send Back
                      </button>
                    </>
                  )}

                  {article.status === 'published' && hasRole('editor', 'admin') && (
                    <button
                      type="button"
                      onClick={() => handleArchive(article.id)}
                      disabled={actionLoading}
                      className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-600 hover:text-slate-900 bg-white border border-slate-200 hover:bg-slate-50 px-3 py-1.5 rounded-xl transition disabled:opacity-50"
                    >
                      <Archive className="h-3.5 w-3.5" />
                      Archive
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Send Back with Feedback Modal */}
      {sendBackArticle && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 backdrop-blur-sm p-4 animate-in fade-in">
          <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl border border-slate-200">
            <h3 className="text-base font-extrabold text-slate-900 mb-2">
              Send Back to Reporter
            </h3>
            <p className="text-xs text-slate-500 mb-4">
              Return article &quot;{sendBackArticle.title}&quot; to draft status with editorial guidance.
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
                onClick={() => setSendBackArticle(null)}
                className="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50 transition"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleConfirmSendBack}
                disabled={actionLoading}
                className="rounded-xl bg-amber-500 hover:bg-amber-600 px-4 py-2 text-xs font-extrabold text-slate-950 transition disabled:opacity-50"
              >
                {actionLoading ? 'Sending...' : 'Confirm Return'}
              </button>
            </div>
          </div>
        </div>
      )}
    </CMSLayout>
  );
}

export default function ArticlesPage() {
  return (
    <ProtectedRoute allowedRoles={['reporter', 'editor', 'admin']}>
      <Suspense fallback={<div className="p-8 text-center text-slate-400">Loading articles workspace...</div>}>
        <ArticlesContent />
      </Suspense>
    </ProtectedRoute>
  );
}
