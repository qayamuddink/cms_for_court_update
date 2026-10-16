'use client';

import React from 'react';
import ProtectedRoute from '@/components/auth/ProtectedRoute';
import CMSLayout from '@/components/layout/CMSLayout';
import ArticleEditorForm from '@/components/articles/ArticleEditorForm';

export default function NewArticlePage() {
  return (
    <ProtectedRoute allowedRoles={['reporter', 'editor', 'admin']}>
      <CMSLayout title="Compose New Article" badge="Drafting">
        <ArticleEditorForm isNew={true} />
      </CMSLayout>
    </ProtectedRoute>
  );
}
