'use client';

import React, { useState, useEffect } from 'react';
import ProtectedRoute from '@/components/auth/ProtectedRoute';
import CMSLayout from '@/components/layout/CMSLayout';
import { apiClient } from '@/lib/api/client';
import {
  Search,
  RefreshCw,
  Database,
  FileText,
  BookOpen,
  Briefcase,
  Activity,
  AlertTriangle,
  Clock,
  Sparkles,
  CheckCircle2,
  Lock,
  Layers,
  FileSearch,
} from 'lucide-react';

interface SearchHealth {
  total_documents: number;
  article_count: number;
  judgment_count: number;
  case_count: number;
  engine: string;
  status: string;
  last_sync_at?: string;
}

interface Analytics {
  total_searches: number;
  top_queries: Array<{ query: string; count: number }>;
  zero_result_queries: Array<{ query: string; searched_at: string; has_filters: boolean }>;
}

export default function SearchHealthPage() {
  const [health, setHealth] = useState<SearchHealth | null>(null);
  const [analytics, setAnalytics] = useState<Analytics | null>(null);
  const [loading, setLoading] = useState(true);
  const [reindexing, setReindexing] = useState(false);
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  // Search Simulator state
  const [simQuery, setSimQuery] = useState('');
  const [simDocType, setSimDocType] = useState('all');
  const [simCourt, setSimCourt] = useState('');
  const [simStatute, setSimStatute] = useState('');
  const [simSectionCited, setSimSectionCited] = useState('');
  const [simResults, setSimResults] = useState<any | null>(null);
  const [simSearching, setSimSearching] = useState(false);

  const fetchHealthAndAnalytics = async () => {
    try {
      setLoading(true);
      const [healthRes, analyticsRes] = await Promise.all([
        apiClient.GET('/v1/cms/search/health'),
        apiClient.GET('/v1/cms/search/analytics', { params: { query: { limit: 15 } } }),
      ]);

      if (healthRes.data) {
        setHealth(healthRes.data as SearchHealth);
      }
      if (analyticsRes.data) {
        setAnalytics(analyticsRes.data as Analytics);
      }
    } catch (err) {
      console.error('Failed to load search health:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchHealthAndAnalytics();
  }, []);

  const handleReindex = async () => {
    try {
      setReindexing(true);
      const res = await apiClient.POST('/v1/cms/search/reindex');
      if (res.data) {
        setToastMessage('Full search index rebuild completed successfully!');
        await fetchHealthAndAnalytics();
      }
    } catch (err) {
      console.error('Re-index failed:', err);
      setToastMessage('Re-index operation failed. Check server logs.');
    } finally {
      setReindexing(false);
      setTimeout(() => setToastMessage(null), 4000);
    }
  };

  const handleSimSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!simQuery && !simStatute && !simSectionCited) return;

    try {
      setSimSearching(true);
      const res = await apiClient.GET('/v1/search', {
        params: {
          query: {
            q: simQuery || undefined,
            type: simDocType !== 'all' ? (simDocType as any) : undefined,
            court: simCourt || undefined,
            statute: simStatute || undefined,
            section_cited: simSectionCited || undefined,
            limit: 10,
          },
        },
      });
      if (res.data) {
        setSimResults(res.data);
      }
    } catch (err) {
      console.error('Sim search error:', err);
    } finally {
      setSimSearching(false);
    }
  };

  return (
    <ProtectedRoute allowedRoles={['editor', 'admin']}>
      <CMSLayout title="Search Health">
        <div className="space-y-8 max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-slate-800 pb-6">
            <div>
              <div className="flex items-center gap-2 text-amber-500 font-semibold text-xs tracking-wider uppercase mb-1">
                <Database className="h-4 w-4" /> Search &amp; Index Engine
              </div>
              <h1 className="text-2xl font-bold text-white tracking-tight sm:text-3xl">
                Search Health &amp; Analytics
              </h1>
              <p className="text-sm text-slate-400 mt-1">
                Monitor index metrics, full-text PDF search status, query gap analytics, and manage re-indexing.
              </p>
            </div>
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={handleReindex}
                disabled={reindexing}
                className="inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 text-slate-950 font-semibold text-sm hover:from-amber-400 hover:to-amber-500 transition-all shadow-lg shadow-amber-500/20 disabled:opacity-60 cursor-pointer"
              >
                <RefreshCw className={`h-4 w-4 ${reindexing ? 'animate-spin' : ''}`} />
                {reindexing ? 'Re-indexing Documents...' : 'Re-index All Documents'}
              </button>
            </div>
          </div>

          {/* Toast Notification */}
          {toastMessage && (
            <div className="rounded-xl border border-emerald-500/30 bg-emerald-950/60 p-4 text-emerald-200 text-sm flex items-center gap-3 animate-fade-in shadow-lg">
              <CheckCircle2 className="h-5 w-5 text-emerald-400 shrink-0" />
              <span>{toastMessage}</span>
            </div>
          )}

          {/* Key Metric Cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
            {/* Status Card */}
            <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5 backdrop-blur-sm">
              <div className="flex items-center justify-between text-slate-400 mb-3">
                <span className="text-xs font-semibold uppercase tracking-wider">Index Engine</span>
                <div className="flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-xs font-medium">
                  <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
                  {health?.status || 'Active'}
                </div>
              </div>
              <div className="text-xl font-bold text-white capitalize">
                {health?.engine || 'Meilisearch'}
              </div>
              <div className="text-xs text-slate-400 mt-2 flex items-center gap-1.5">
                <Clock className="h-3.5 w-3.5 text-slate-500" />
                Last sync:{' '}
                {health?.last_sync_at ? new Date(health.last_sync_at).toLocaleTimeString() : 'Just now'}
              </div>
            </div>

            {/* Total Documents */}
            <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5 backdrop-blur-sm">
              <div className="flex items-center justify-between text-slate-400 mb-3">
                <span className="text-xs font-semibold uppercase tracking-wider">Total Indexed</span>
                <Layers className="h-5 w-5 text-amber-400" />
              </div>
              <div className="text-3xl font-extrabold text-white">
                {health?.total_documents ?? '—'}
              </div>
              <div className="text-xs text-slate-400 mt-2">
                Across all searchable platforms
              </div>
            </div>

            {/* Judgments with PDF Text */}
            <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5 backdrop-blur-sm">
              <div className="flex items-center justify-between text-slate-400 mb-3">
                <span className="text-xs font-semibold uppercase tracking-wider">Judgments (PDF Text)</span>
                <BookOpen className="h-5 w-5 text-blue-400" />
              </div>
              <div className="text-3xl font-extrabold text-white">
                {health?.judgment_count ?? '—'}
              </div>
              <div className="text-xs text-blue-400 mt-2 font-medium flex items-center gap-1">
                <FileSearch className="h-3.5 w-3.5" /> Includes OCR page text
              </div>
            </div>

            {/* Articles & Cases */}
            <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5 backdrop-blur-sm">
              <div className="flex items-center justify-between text-slate-400 mb-3">
                <span className="text-xs font-semibold uppercase tracking-wider">Articles &amp; Cases</span>
                <FileText className="h-5 w-5 text-purple-400" />
              </div>
              <div className="flex items-baseline gap-2">
                <span className="text-2xl font-bold text-white">{health?.article_count ?? '—'}</span>
                <span className="text-xs text-slate-400">articles /</span>
                <span className="text-2xl font-bold text-white">{health?.case_count ?? '—'}</span>
                <span className="text-xs text-slate-400">cases</span>
              </div>
              <div className="text-xs text-slate-400 mt-2">
                Synced via transactional outbox
              </div>
            </div>
          </div>

          {/* Interactive Search Simulator & Inspector */}
          <div className="rounded-2xl border border-slate-800 bg-slate-900/70 p-6 backdrop-blur-sm">
            <div className="flex items-center justify-between mb-4 pb-3 border-b border-slate-800">
              <div className="flex items-center gap-2">
                <Sparkles className="h-5 w-5 text-amber-400" />
                <h2 className="text-base font-bold text-white">Interactive Search Simulator</h2>
              </div>
              <span className="text-xs text-slate-400">Test query ranking, statutory filters &amp; highlighted snippets</span>
            </div>

            <form onSubmit={handleSimSearch} className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
                <div className="md:col-span-2 relative">
                  <Search className="absolute left-3.5 top-3.5 h-4 w-4 text-slate-400" />
                  <input
                    type="text"
                    value={simQuery}
                    onChange={(e) => setSimQuery(e.target.value)}
                    placeholder="Search keywords, case title, or exact citation..."
                    className="w-full pl-10 pr-4 py-2.5 rounded-xl bg-slate-950/80 border border-slate-700 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-amber-500"
                  />
                </div>
                <div>
                  <select
                    value={simDocType}
                    onChange={(e) => setSimDocType(e.target.value)}
                    className="w-full px-3 py-2.5 rounded-xl bg-slate-950/80 border border-slate-700 text-sm text-white focus:outline-none focus:border-amber-500"
                  >
                    <option value="all">All Document Types</option>
                    <option value="judgment">Judgments Only</option>
                    <option value="article">Articles Only</option>
                    <option value="court_case">Court Cases Only</option>
                  </select>
                </div>
                <div>
                  <input
                    type="text"
                    value={simCourt}
                    onChange={(e) => setSimCourt(e.target.value)}
                    placeholder="Court filter (e.g. Supreme Court)..."
                    className="w-full px-3.5 py-2.5 rounded-xl bg-slate-950/80 border border-slate-700 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-amber-500"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                <div>
                  <input
                    type="text"
                    value={simStatute}
                    onChange={(e) => setSimStatute(e.target.value)}
                    placeholder="Statute (e.g. Arbitration and Conciliation Act)..."
                    className="w-full px-3.5 py-2.5 rounded-xl bg-slate-950/80 border border-slate-700 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-amber-500"
                  />
                </div>
                <div>
                  <input
                    type="text"
                    value={simSectionCited}
                    onChange={(e) => setSimSectionCited(e.target.value)}
                    placeholder="Section cited (e.g. Section 138, Article 14)..."
                    className="w-full px-3.5 py-2.5 rounded-xl bg-slate-950/80 border border-slate-700 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-amber-500"
                  />
                </div>
                <div className="flex items-center gap-2">
                  <button
                    type="submit"
                    disabled={simSearching}
                    className="flex-1 py-2.5 px-4 rounded-xl bg-slate-800 text-white font-medium text-sm hover:bg-slate-700 border border-slate-700 transition-colors cursor-pointer"
                  >
                    {simSearching ? 'Searching...' : 'Run Test Query'}
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      setSimQuery('');
                      setSimCourt('');
                      setSimStatute('');
                      setSimSectionCited('');
                      setSimResults(null);
                    }}
                    className="py-2.5 px-3 rounded-xl bg-slate-950 text-slate-400 hover:text-white border border-slate-800 text-sm cursor-pointer"
                  >
                    Clear
                  </button>
                </div>
              </div>
            </form>

            {/* Results Preview */}
            {simResults && (
              <div className="mt-6 pt-5 border-t border-slate-800/80 space-y-4">
                <div className="flex items-center justify-between text-xs text-slate-400">
                  <span>Found {simResults.total_hits} results in {simResults.processing_time_ms}ms</span>
                  {simResults.is_upgrade_required && (
                    <span className="flex items-center gap-1 text-amber-400 font-medium">
                      <Lock className="h-3.5 w-3.5" /> Requires Premium Entitlement
                    </span>
                  )}
                </div>

                {simResults.exact_citation_match && (
                  <div className="p-4 rounded-xl border border-emerald-500/40 bg-emerald-950/30 flex items-start justify-between gap-3">
                    <div>
                      <div className="text-xs font-bold uppercase tracking-wider text-emerald-400 mb-1">
                        Exact Citation Match
                      </div>
                      <div className="font-semibold text-white">{simResults.exact_citation_match.title}</div>
                      <div className="text-xs font-mono text-emerald-300 mt-0.5">
                        {simResults.exact_citation_match.citation}
                      </div>
                    </div>
                  </div>
                )}

                <div className="space-y-3">
                  {simResults.hits.map((item: any) => (
                    <div
                      key={item.id}
                      className="p-4 rounded-xl border border-slate-800/80 bg-slate-950/40 hover:border-slate-700 transition-colors"
                    >
                      <div className="flex flex-wrap items-center gap-2 mb-1.5">
                        <span className="text-[10px] font-bold px-2 py-0.5 rounded-full uppercase tracking-wider bg-slate-800 text-slate-300 border border-slate-700">
                          {item.document_type}
                        </span>
                        {item.citation && (
                          <span className="text-xs font-mono font-semibold text-amber-400">
                            {item.citation}
                          </span>
                        )}
                        {item.matched_page_number && (
                          <span className="text-xs font-medium text-blue-400 bg-blue-500/10 px-2 py-0.5 rounded border border-blue-500/20 flex items-center gap-1">
                            <FileText className="h-3 w-3" /> Page {item.matched_page_number}
                          </span>
                        )}
                        {item.court && (
                          <span className="text-xs text-slate-400">{item.court}</span>
                        )}
                      </div>
                      <div className="text-sm font-semibold text-white mb-1.5">{item.title}</div>
                      {item.snippet && (
                        <div
                          className="text-xs text-slate-300 leading-relaxed font-normal bg-slate-900/60 p-2.5 rounded-lg border border-slate-800"
                          dangerouslySetInnerHTML={{ __html: item.snippet }}
                        />
                      )}
                    </div>
                  ))}
                  {simResults.hits.length === 0 && (
                    <div className="py-8 text-center text-sm text-slate-500">
                      No results matched your search query or filters.
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>

          {/* Search Analytics: Top Queries & Zero Results */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Top Queries */}
            <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6 backdrop-blur-sm">
              <div className="flex items-center justify-between mb-4 pb-3 border-b border-slate-800">
                <div className="flex items-center gap-2">
                  <Activity className="h-5 w-5 text-emerald-400" />
                  <h3 className="font-bold text-white text-base">Top Search Queries</h3>
                </div>
                <span className="text-xs text-slate-400">Highest reader demand</span>
              </div>

              <div className="space-y-2.5">
                {analytics?.top_queries?.map((item, idx) => (
                  <div
                    key={idx}
                    className="flex items-center justify-between p-3 rounded-xl bg-slate-950/40 border border-slate-800/80"
                  >
                    <div className="flex items-center gap-3">
                      <span className="text-xs font-mono font-bold text-slate-500 w-5">#{idx + 1}</span>
                      <span className="text-sm font-medium text-slate-200">{item.query}</span>
                    </div>
                    <span className="text-xs font-semibold px-2.5 py-1 rounded-full bg-slate-800 text-slate-300">
                      {item.count} searches
                    </span>
                  </div>
                ))}
                {(!analytics?.top_queries || analytics.top_queries.length === 0) && (
                  <div className="py-6 text-center text-xs text-slate-500">
                    No search queries recorded yet.
                  </div>
                )}
              </div>
            </div>

            {/* Zero-Result Queries */}
            <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6 backdrop-blur-sm">
              <div className="flex items-center justify-between mb-4 pb-3 border-b border-slate-800">
                <div className="flex items-center gap-2">
                  <AlertTriangle className="h-5 w-5 text-amber-400" />
                  <h3 className="font-bold text-white text-base">Zero-Result Searches</h3>
                </div>
                <span className="text-xs text-slate-400">Editorial gap alerts</span>
              </div>

              <div className="space-y-2.5">
                {analytics?.zero_result_queries?.map((item, idx) => (
                  <div
                    key={idx}
                    className="flex items-center justify-between p-3 rounded-xl bg-slate-950/40 border border-slate-800/80"
                  >
                    <div>
                      <div className="text-sm font-medium text-amber-200/90">{item.query}</div>
                      <div className="text-[11px] text-slate-500 mt-0.5">
                        {new Date(item.searched_at).toLocaleDateString()} at{' '}
                        {new Date(item.searched_at).toLocaleTimeString()}
                      </div>
                    </div>
                    {item.has_filters && (
                      <span className="text-[10px] font-medium px-2 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20">
                        Filtered
                      </span>
                    )}
                  </div>
                ))}
                {(!analytics?.zero_result_queries || analytics.zero_result_queries.length === 0) && (
                  <div className="py-6 text-center text-xs text-slate-500">
                    No zero-result queries recorded.
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </CMSLayout>
    </ProtectedRoute>
  );
}
