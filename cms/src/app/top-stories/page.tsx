'use client';

import React, { useState, useEffect } from 'react';
import CMSLayout from '@/components/layout/CMSLayout';
import apiClient from '@/lib/api/client';
import { useAuth } from '@/lib/auth/context';
import {
  Flame,
  Star,
  ArrowUp,
  ArrowDown,
  Save,
  CheckCircle2,
  AlertCircle,
  Loader2,
  FileText,
  Clock,
  Sparkles,
} from 'lucide-react';

interface ArticleItem {
  id: string;
  title: string;
  slug: string;
  article_type: string;
  category: string;
  status: string;
  is_pinned_top_story?: boolean;
  top_story_priority?: number;
  published_at?: string | null;
}

export default function TopStoriesPage() {
  const { hasRole } = useAuth();
  const [articles, setArticles] = useState<ArticleItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(
    null
  );

  const fetchData = async () => {
    try {
      setLoading(true);
      setStatusMessage(null);
      // Fetch published and active articles
      const { data, error, response } = await apiClient.GET('/v1/cms/articles', {
        params: { query: { status: 'published', limit: 50 } },
      });
      if (response.ok && data) {
        const raw = (data as any).articles || [];
        // Sort: pinned first by priority ASC, then unpinned by published_at DESC
        const sorted = [...raw].sort((a: any, b: any) => {
          if (a.is_pinned_top_story && !b.is_pinned_top_story) return -1;
          if (!a.is_pinned_top_story && b.is_pinned_top_story) return 1;
          if (a.is_pinned_top_story && b.is_pinned_top_story) {
            return (a.top_story_priority || 1) - (b.top_story_priority || 1);
          }
          return 0;
        });
        setArticles(sorted);
      } else {
        setStatusMessage({
          text: (error as any)?.message || 'Failed to fetch articles.',
          type: 'error',
        });
      }
    } catch (err: any) {
      setStatusMessage({ text: err.message || 'Network error fetching articles.', type: 'error' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const togglePin = (id: string) => {
    setArticles((prev) =>
      prev.map((art) => {
        if (art.id === id) {
          const nextPinned = !art.is_pinned_top_story;
          return {
            ...art,
            is_pinned_top_story: nextPinned,
            top_story_priority: nextPinned ? (art.top_story_priority || 1) : 0,
          };
        }
        return art;
      })
    );
  };

  const moveItem = (index: number, direction: 'up' | 'down') => {
    const targetIndex = direction === 'up' ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= articles.length) return;

    const newArticles = [...articles];
    const [moved] = newArticles.splice(index, 1);
    newArticles.splice(targetIndex, 0, moved);

    // Re-assign priorities for pinned items
    let currentPri = 1;
    newArticles.forEach((art) => {
      if (art.is_pinned_top_story) {
        art.top_story_priority = currentPri++;
      }
    });

    setArticles(newArticles);
  };

  const setPriorityDirectly = (id: string, priority: number) => {
    setArticles((prev) =>
      prev.map((art) => (art.id === id ? { ...art, top_story_priority: priority } : art))
    );
  };

  const handleSaveOrder = async () => {
    setSaving(true);
    setStatusMessage(null);
    try {
      const items = articles.map((art, idx) => ({
        article_id: art.id,
        priority: art.is_pinned_top_story ? art.top_story_priority || idx + 1 : 0,
        is_pinned: !!art.is_pinned_top_story,
      }));

      const { data, error, response } = await apiClient.PUT('/v1/cms/top-stories', {
        body: { items },
      });

      if (response.ok) {
        setStatusMessage({ text: 'Top stories order saved successfully!', type: 'success' });
        fetchData();
      } else {
        setStatusMessage({
          text: (error as any)?.message || 'Failed to update top stories order.',
          type: 'error',
        });
      }
    } catch (err: any) {
      setStatusMessage({ text: err.message || 'Network error saving order.', type: 'error' });
    } finally {
      setSaving(false);
    }
  };

  const pinnedCount = articles.filter((a) => a.is_pinned_top_story).length;

  return (
    <CMSLayout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
              <Flame className="h-6 w-6 text-amber-500" />
              Featured &amp; Top Stories
            </h1>
            <p className="text-sm text-slate-400 mt-1">
              Pin crucial breaking updates and landmark rulings to the homepage hero grid.
            </p>
          </div>
          {hasRole('editor', 'admin') && (
            <button
              onClick={handleSaveOrder}
              disabled={saving || loading}
              className="inline-flex items-center justify-center gap-2 px-5 py-2.5 bg-amber-500 hover:bg-amber-400 text-slate-950 font-bold text-sm rounded-xl transition shadow-lg shadow-amber-500/15 disabled:opacity-50"
            >
              {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              Save Top Stories Order
            </button>
          )}
        </div>

        {/* Status Message */}
        {statusMessage && (
          <div
            className={`p-4 rounded-xl flex items-center gap-3 text-sm border ${
              statusMessage.type === 'success'
                ? 'bg-emerald-500/10 border-emerald-500/20 text-emerald-400'
                : 'bg-red-500/10 border-red-500/20 text-red-400'
            }`}
          >
            {statusMessage.type === 'success' ? (
              <CheckCircle2 className="h-5 w-5 shrink-0" />
            ) : (
              <AlertCircle className="h-5 w-5 shrink-0" />
            )}
            <span>{statusMessage.text}</span>
          </div>
        )}

        {/* Overview Bar */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-4 flex items-center gap-4">
            <div className="h-10 w-10 rounded-xl bg-amber-500/10 flex items-center justify-center text-amber-400 border border-amber-500/20">
              <Star className="h-5 w-5" />
            </div>
            <div>
              <p className="text-xs uppercase tracking-wider text-slate-400 font-semibold">Pinned Top Stories</p>
              <p className="text-xl font-bold text-white mt-0.5">{pinnedCount}</p>
            </div>
          </div>
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-4 flex items-center gap-4">
            <div className="h-10 w-10 rounded-xl bg-blue-500/10 flex items-center justify-center text-blue-400 border border-blue-500/20">
              <FileText className="h-5 w-5" />
            </div>
            <div>
              <p className="text-xs uppercase tracking-wider text-slate-400 font-semibold">Published Feed Articles</p>
              <p className="text-xl font-bold text-white mt-0.5">{articles.length}</p>
            </div>
          </div>
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-4 flex items-center gap-4">
            <div className="h-10 w-10 rounded-xl bg-purple-500/10 flex items-center justify-center text-purple-400 border border-purple-500/20">
              <Sparkles className="h-5 w-5" />
            </div>
            <div>
              <p className="text-xs uppercase tracking-wider text-slate-400 font-semibold">Hero Slot 1</p>
              <p className="text-sm font-semibold text-amber-400 truncate mt-0.5">
                {articles.find((a) => a.is_pinned_top_story)?.title || 'No story pinned'}
              </p>
            </div>
          </div>
        </div>

        {/* Stories List */}
        <div className="bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
          {loading ? (
            <div className="py-20 flex flex-col items-center justify-center text-slate-400 gap-3">
              <Loader2 className="h-8 w-8 animate-spin text-amber-500" />
              <p className="text-sm">Loading articles for top story placement...</p>
            </div>
          ) : articles.length === 0 ? (
            <div className="py-16 text-center text-slate-400">
              <FileText className="h-10 w-10 mx-auto text-slate-600 mb-2" />
              <p className="text-base font-semibold text-white">No published articles found</p>
              <p className="text-sm text-slate-500 mt-1">
                Publish articles from the Editorial Desk to feature them as Top Stories.
              </p>
            </div>
          ) : (
            <div className="divide-y divide-slate-800">
              {articles.map((art, idx) => {
                const isPinned = !!art.is_pinned_top_story;
                return (
                  <div
                    key={art.id}
                    className={`flex items-center gap-4 p-4 transition ${
                      isPinned ? 'bg-amber-500/5 hover:bg-amber-500/10' : 'hover:bg-slate-850/50'
                    }`}
                  >
                    {/* Pin Toggle */}
                    <button
                      type="button"
                      onClick={() => togglePin(art.id)}
                      className={`p-2 rounded-xl transition ${
                        isPinned
                          ? 'bg-amber-500 text-slate-950 shadow-md shadow-amber-500/20'
                          : 'bg-slate-800 text-slate-400 hover:text-white'
                      }`}
                      title={isPinned ? 'Unpin from Top Stories' : 'Pin to Top Stories'}
                    >
                      <Star className="h-5 w-5 fill-current" />
                    </button>

                    {/* Ordering buttons */}
                    <div className="flex flex-col gap-1">
                      <button
                        type="button"
                        onClick={() => moveItem(idx, 'up')}
                        disabled={idx === 0}
                        className="p-1 rounded text-slate-400 hover:text-white hover:bg-slate-800 disabled:opacity-20"
                        title="Move Up"
                      >
                        <ArrowUp className="h-3.5 w-3.5" />
                      </button>
                      <button
                        type="button"
                        onClick={() => moveItem(idx, 'down')}
                        disabled={idx === articles.length - 1}
                        className="p-1 rounded text-slate-400 hover:text-white hover:bg-slate-800 disabled:opacity-20"
                        title="Move Down"
                      >
                        <ArrowDown className="h-3.5 w-3.5" />
                      </button>
                    </div>

                    {/* Priority input if pinned */}
                    {isPinned ? (
                      <div className="flex flex-col items-center">
                        <span className="text-[10px] text-amber-400 font-mono font-bold uppercase">Priority</span>
                        <input
                          type="number"
                          min={1}
                          max={99}
                          value={art.top_story_priority || idx + 1}
                          onChange={(e) => setPriorityDirectly(art.id, parseInt(e.target.value) || 1)}
                          className="w-12 bg-slate-950 border border-amber-500/40 text-amber-300 font-mono text-center text-xs py-1 rounded-lg focus:outline-none"
                        />
                      </div>
                    ) : (
                      <div className="w-12 text-center text-xs text-slate-600 font-mono font-semibold">
                        —
                      </div>
                    )}

                    {/* Article Details */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1">
                        <span className="px-2 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wider bg-slate-800 text-slate-300">
                          {art.article_type.replace('_', ' ')}
                        </span>
                        <span className="px-2 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wider bg-slate-800/60 text-slate-400">
                          {art.category}
                        </span>
                        {isPinned && (
                          <span className="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-amber-500/20 text-amber-400 border border-amber-500/30">
                            PINNED #{art.top_story_priority || idx + 1}
                          </span>
                        )}
                      </div>
                      <h3 className="font-semibold text-white text-sm line-clamp-1">{art.title}</h3>
                      <p className="text-xs text-slate-500 font-mono mt-0.5">/{art.slug}</p>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </CMSLayout>
  );
}
