'use client';

import React, { useState, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import apiClient from '@/lib/api/client';
import ProtectedRoute from '@/components/auth/ProtectedRoute';
import CMSLayout from '@/components/layout/CMSLayout';
import ArticleEditorForm from '@/components/articles/ArticleEditorForm';
import { Loader2, AlertCircle, ArrowLeft } from 'lucide-react';

export default function EditArticlePage() {
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;

  const [article, setArticle] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;

    const fetchArticle = async () => {
      setLoading(true);
      setError(null);
      try {
        const { data, response, error } = await apiClient.GET('/v1/cms/articles/{id}', {
          params: { path: { id } },
        });

        if (!response.ok || !data) {
          throw new Error((error as any)?.message || 'Failed to load article');
        }

        setArticle(data);
      } catch (err: any) {
        setError(err.message || 'Error loading article details');
      } finally {
        setLoading(false);
      }
    };

    fetchArticle();
  }, [id]);

  return (
    <ProtectedRoute allowedRoles={['reporter', 'editor', 'admin']}>
      <CMSLayout title="Edit Article" badge={article ? `v${article.version}` : 'Loading'}>
        {loading ? (
          <div className="py-24 text-center text-slate-400">
            <Loader2 className="h-8 w-8 animate-spin mx-auto mb-3 text-amber-500" />
            <p className="text-xs font-semibold">Retrieving article content and version...</p>
          </div>
        ) : error || !article ? (
          <div className="rounded-2xl bg-white border border-red-200 p-8 text-center max-w-md mx-auto my-12 shadow-sm">
            <AlertCircle className="h-10 w-10 text-red-500 mx-auto mb-3" />
            <h3 className="text-base font-extrabold text-slate-900 mb-1">
              Article Not Found
            </h3>
            <p className="text-xs text-slate-500 mb-4">{error || 'Could not find the requested article.'}</p>
            <button
              type="button"
              onClick={() => router.push('/articles')}
              className="inline-flex items-center gap-1.5 rounded-xl bg-slate-900 px-4 py-2 text-xs font-bold text-white transition hover:bg-slate-800"
            >
              <ArrowLeft className="h-4 w-4" />
              Back to Articles
            </button>
          </div>
        ) : (
          <ArticleEditorForm initialData={article} isNew={false} />
        )}
      </CMSLayout>
    </ProtectedRoute>
  );
}
