'use client';

import React, { useEffect, useState } from 'react';
import ProtectedRoute from '@/components/auth/ProtectedRoute';
import CMSLayout from '@/components/layout/CMSLayout';
import { useAuth } from '@/lib/auth/context';
import apiClient from '@/lib/api/client';
import {
  Radio,
  FileEdit,
  Clock,
  ArrowRight,
  Scale,
  RefreshCw,
  Loader2,
  Calendar,
  UserCheck,
} from 'lucide-react';

export default function DashboardPage() {
  const { user, hasRole } = useAuth();
  const [liveCases, setLiveCases] = useState<any[]>([]);
  const [myDrafts, setMyDrafts] = useState<any[]>([]);
  const [reviewCount, setReviewCount] = useState<number>(0);
  const [loading, setLoading] = useState(true);

  const fetchDashboardData = async () => {
    setLoading(true);
    try {
      // 1. Fetch live cases
      const { data: casesData } = await apiClient.GET('/v1/cases/live');
      const items = (casesData as any)?.cases || (Array.isArray(casesData) ? casesData : []);
      if (Array.isArray(items)) {
        setLiveCases(items);
      }

      // 2. Fetch my drafts (CMS articles with draft status)
      if (hasRole('reporter', 'editor', 'admin')) {
        const { data: draftsData } = await apiClient.GET('/v1/cms/articles', {
          params: { query: { status: 'draft', limit: 5 } },
        });
        if (draftsData?.articles) {
          setMyDrafts(draftsData.articles);
        }
      }

      // 3. Fetch review queue count (for editor & admin)
      if (hasRole('editor', 'admin')) {
        const { data: reviewData } = await apiClient.GET('/v1/cms/articles', {
          params: { query: { status: 'in_review', limit: 1 } },
        });
        if (reviewData) {
          setReviewCount(reviewData.total || 0);
        }
      }
    } catch {
      // Ignore background fetch error
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDashboardData();
  }, []);

  return (
    <ProtectedRoute allowedRoles={['reporter', 'editor', 'admin']}>
      <CMSLayout title="Editorial Dashboard" badge="Overview">
        {/* Welcome Banner */}
        <div className="relative overflow-hidden rounded-2xl bg-gradient-to-r from-slate-900 via-slate-850 to-slate-900 border border-slate-800 p-6 sm:p-8 text-white shadow-xl mb-8">
          <div className="relative z-10 flex flex-col md:flex-row md:items-center md:justify-between gap-6">
            <div>
              <div className="flex items-center gap-2 mb-2">
                <span className="text-xs uppercase tracking-wider font-semibold text-amber-400 bg-amber-400/10 px-2.5 py-0.5 rounded-full border border-amber-400/20">
                  {user?.system_role} Station
                </span>
                <span className="text-slate-400 text-xs">
                  {new Date().toLocaleDateString('en-IN', { weekday: 'long', month: 'short', day: 'numeric', year: 'numeric' })}
                </span>
              </div>
              <h2 className="text-2xl sm:text-3xl font-extrabold tracking-tight">
                Welcome back, {user?.full_name}
              </h2>
              <p className="mt-1 text-sm text-slate-300 max-w-xl">
                Real-time court reporting desk and editorial publishing pipeline. Keep legal reporting accurate, timely, and verified.
              </p>
            </div>
            <div className="flex items-center gap-3">
              <a
                href="/desk"
                className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 px-5 py-3 text-sm font-bold text-slate-950 shadow-lg shadow-amber-500/20 hover:from-amber-400 hover:to-amber-500 transition-all"
              >
                <Radio className="h-4 w-4 text-slate-950 animate-pulse" />
                Live Court Desk
              </a>
              <button
                onClick={fetchDashboardData}
                disabled={loading}
                className="p-3 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
                title="Refresh dashboard"
              >
                <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
              </button>
            </div>
          </div>
        </div>

        {/* Metric Cards Grid */}
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3 mb-8">
          {/* Card 1: Live Cases Today */}
          <div className="relative overflow-hidden rounded-2xl bg-white border border-slate-200/80 p-6 shadow-sm hover:shadow-md transition">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs font-bold uppercase tracking-wider text-slate-500">Live Court Hearings</p>
                <p className="mt-2 text-3xl font-black text-slate-900 tracking-tight">
                  {loading ? '...' : liveCases.length}
                </p>
              </div>
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-red-50 text-red-600 border border-red-200">
                <Radio className="h-6 w-6 animate-pulse" />
              </div>
            </div>
            <div className="mt-4 flex items-center gap-2 text-xs font-semibold text-slate-600">
              <span className="flex h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
              Streaming live updates now
            </div>
          </div>

          {/* Card 2: My Drafts */}
          <div className="relative overflow-hidden rounded-2xl bg-white border border-slate-200/80 p-6 shadow-sm hover:shadow-md transition">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs font-bold uppercase tracking-wider text-slate-500">My Working Drafts</p>
                <p className="mt-2 text-3xl font-black text-slate-900 tracking-tight">
                  {loading ? '...' : myDrafts.length}
                </p>
              </div>
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-50 text-amber-600 border border-amber-200">
                <FileEdit className="h-6 w-6" />
              </div>
            </div>
            <div className="mt-4 flex items-center text-xs font-semibold text-slate-600">
              <span className="text-slate-500">Saved in editorial workspace</span>
            </div>
          </div>

          {/* Card 3: Review Queue */}
          <div className="relative overflow-hidden rounded-2xl bg-white border border-slate-200/80 p-6 shadow-sm hover:shadow-md transition">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs font-bold uppercase tracking-wider text-slate-500">Review Queue</p>
                <p className="mt-2 text-3xl font-black text-slate-900 tracking-tight">
                  {loading ? '...' : reviewCount}
                </p>
              </div>
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-blue-50 text-blue-600 border border-blue-200">
                <Clock className="h-6 w-6" />
              </div>
            </div>
            <div className="mt-4 flex items-center text-xs font-semibold text-slate-600">
              {hasRole('editor', 'admin') ? (
                <span className="text-blue-600 font-medium">Pending editorial approval</span>
              ) : (
                <span className="text-slate-400">Editor access only</span>
              )}
            </div>
          </div>
        </div>

        {/* Active Live Cases Section */}
        <div className="bg-white rounded-2xl border border-slate-200/80 shadow-sm overflow-hidden mb-8">
          <div className="px-6 py-4 border-b border-slate-200 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-red-100 text-red-600">
                <Radio className="h-4 w-4" />
              </div>
              <div>
                <h3 className="font-bold text-slate-900 text-base">Active Live Cases Today</h3>
                <p className="text-xs text-slate-500">Cases currently configured for live courtroom ticker reporting</p>
              </div>
            </div>
            <a
              href="/desk"
              className="text-xs font-semibold text-amber-600 hover:text-amber-700 flex items-center gap-1"
            >
              Open Live Desk
              <ArrowRight className="h-3.5 w-3.5" />
            </a>
          </div>

          <div className="divide-y divide-slate-100">
            {loading ? (
              <div className="p-8 text-center text-slate-400">
                <Loader2 className="h-6 w-6 animate-spin mx-auto mb-2 text-amber-500" />
                Loading live court cases...
              </div>
            ) : liveCases.length === 0 ? (
              <div className="p-8 text-center text-slate-500">
                <Scale className="h-10 w-10 text-slate-300 mx-auto mb-3" />
                <p className="font-semibold text-sm">No court cases currently live</p>
                <p className="text-xs text-slate-400 mt-1">
                  Editors can toggle cases to live mode from the Court Cases directory.
                </p>
              </div>
            ) : (
              liveCases.map((c) => (
                <div key={c.id} className="p-6 hover:bg-slate-50/70 transition flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 mb-1.5 flex-wrap">
                      <span className="px-2 py-0.5 rounded text-[11px] font-bold uppercase tracking-wider bg-red-100 text-red-700 flex items-center gap-1">
                        <span className="h-1.5 w-1.5 rounded-full bg-red-600 animate-pulse" />
                        Live
                      </span>
                      <span className="text-xs font-medium text-slate-500 bg-slate-100 px-2 py-0.5 rounded">
                        {c.court_name}
                      </span>
                      <span className="text-xs font-semibold text-slate-700">
                        {c.case_number}
                      </span>
                    </div>
                    <h4 className="text-base font-bold text-slate-900 tracking-tight leading-snug">
                      {c.title}
                    </h4>
                    <p className="text-xs text-slate-500 mt-1">
                      Presiding: <span className="font-medium text-slate-700">{c.presiding_judge || 'Bench'}</span> &bull; Last update #{c.last_sequence_no || 0}
                    </p>
                  </div>
                  <div>
                    <a
                      href={`/desk?caseId=${c.id}`}
                      className="inline-flex items-center gap-2 rounded-xl bg-slate-900 px-4 py-2.5 text-xs font-bold text-white hover:bg-amber-500 hover:text-slate-950 transition shadow-sm"
                    >
                      Report on this Case
                      <ArrowRight className="h-3.5 w-3.5" />
                    </a>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      </CMSLayout>
    </ProtectedRoute>
  );
}
