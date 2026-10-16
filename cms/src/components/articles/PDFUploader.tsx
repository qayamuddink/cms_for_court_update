'use client';

import React, { useState, useRef } from 'react';
import apiClient from '@/lib/api/client';
import { FileUp, FileText, CheckCircle2, AlertCircle, Loader2, Trash2, ExternalLink } from 'lucide-react';

export interface UploadedPDFMeta {
  storage_key: string;
  size_bytes: number;
  checksum: string;
  filename: string;
}

interface PDFUploaderProps {
  currentAttachment?: {
    storage_key?: string | null;
    size_bytes?: number | null;
    checksum?: string | null;
    filename?: string | null;
  } | null;
  articleId?: string;
  onUploadSuccess: (meta: UploadedPDFMeta) => void;
  onRemoveAttachment?: () => void;
  disabled?: boolean;
}

// Helper to compute hex-encoded SHA-256 checksum in browser
async function computeSHA256(file: File): Promise<string> {
  const buffer = await file.arrayBuffer();
  const hashBuffer = await crypto.subtle.digest('SHA-256', buffer);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  return hashArray.map((b) => b.toString(16).padStart(2, '0')).join('');
}

export default function PDFUploader({
  currentAttachment,
  articleId,
  onUploadSuccess,
  onRemoveAttachment,
  disabled = false,
}: PDFUploaderProps) {
  const [uploading, setUploading] = useState(false);
  const [progressText, setProgressText] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (file.type !== 'application/pdf' && !file.name.toLowerCase().endsWith('.pdf')) {
      setError('Only official judgment documents in PDF format are accepted.');
      return;
    }

    if (file.size > 50 * 1024 * 1024) {
      setError('File exceeds maximum upload limit of 50 MB.');
      return;
    }

    setError(null);
    setUploading(true);
    setProgressText('Computing SHA-256 checksum...');

    try {
      // 1. Compute checksum
      const checksum = await computeSHA256(file);

      // 2. Request presigned upload URL from backend
      setProgressText('Requesting secure MinIO presigned URL...');
      const { data: presignData, error: presignError, response: presignRes } = await apiClient.POST(
        '/v1/cms/uploads/presigned-url',
        {
          body: {
            filename: file.name,
            content_type: 'application/pdf',
            size_bytes: file.size,
            purpose: 'judgment_pdf',
          },
        }
      );

      if (!presignRes.ok || !presignData?.upload_url) {
        throw new Error((presignError as any)?.message || 'Failed to obtain presigned upload URL');
      }

      // 3. Direct PUT upload to S3 / MinIO
      setProgressText('Uploading PDF directly to object storage...');
      const uploadRes = await fetch(presignData.upload_url, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/pdf',
        },
        body: file,
      });

      if (!uploadRes.ok) {
        throw new Error(`Object storage rejected upload: HTTP ${uploadRes.status}`);
      }

      // 4. If articleId provided, attach directly to article
      if (articleId) {
        setProgressText('Linking attachment to article...');
        const { error: attachError, response: attachRes } = await apiClient.POST(
          '/v1/cms/articles/{id}/attachment',
          {
            params: { path: { id: articleId } },
            body: {
              storage_key: presignData.storage_key,
              size_bytes: file.size,
              checksum,
            },
          }
        );
        if (!attachRes.ok) {
          throw new Error((attachError as any)?.message || 'Failed to attach PDF to article');
        }
      }

      const meta: UploadedPDFMeta = {
        storage_key: presignData.storage_key,
        size_bytes: file.size,
        checksum,
        filename: file.name,
      };

      setProgressText(null);
      onUploadSuccess(meta);
    } catch (err: any) {
      setError(err.message || 'Error uploading judgment PDF');
    } finally {
      setUploading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    }
  };

  const formatFileSize = (bytes?: number | null) => {
    if (!bytes) return '0 KB';
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  };

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <FileText className="h-5 w-5 text-amber-500" />
          <h4 className="text-sm font-bold text-slate-900">Official Judgment PDF Attachment</h4>
        </div>
        <span className="text-[11px] font-semibold text-slate-400 bg-slate-100 px-2 py-0.5 rounded">
          S3 / MinIO Presigned Direct
        </span>
      </div>

      {error && (
        <div className="mb-4 flex items-center gap-2 rounded-xl bg-red-50 border border-red-200 p-3 text-xs font-medium text-red-700">
          <AlertCircle className="h-4 w-4 shrink-0 text-red-600" />
          <span>{error}</span>
        </div>
      )}

      {currentAttachment?.storage_key ? (
        <div className="rounded-xl border border-emerald-200 bg-emerald-50/50 p-4">
          <div className="flex items-center justify-between flex-wrap gap-2">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-100 text-emerald-700">
                <CheckCircle2 className="h-6 w-6" />
              </div>
              <div>
                <p className="text-xs font-bold text-slate-900 truncate max-w-xs sm:max-w-md">
                  {currentAttachment.filename || currentAttachment.storage_key.split('/').pop()}
                </p>
                <div className="flex items-center gap-2 mt-0.5 text-[11px] text-slate-500">
                  <span>{formatFileSize(currentAttachment.size_bytes)}</span>
                  <span>•</span>
                  <span className="font-mono text-[10px] text-slate-400 truncate max-w-[140px]">
                    SHA-256: {currentAttachment.checksum?.substring(0, 16)}...
                  </span>
                </div>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                disabled={disabled || uploading}
                className="text-xs font-semibold text-slate-700 hover:text-slate-900 bg-white border border-slate-200 hover:bg-slate-50 px-3 py-1.5 rounded-lg transition disabled:opacity-50"
              >
                Replace
              </button>
              {onRemoveAttachment && (
                <button
                  type="button"
                  onClick={onRemoveAttachment}
                  disabled={disabled || uploading}
                  className="text-xs font-semibold text-red-600 hover:text-red-700 bg-white border border-red-200 hover:bg-red-50 p-1.5 rounded-lg transition disabled:opacity-50"
                  title="Remove attachment"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              )}
            </div>
          </div>
        </div>
      ) : (
        <div
          onClick={() => !disabled && !uploading && fileInputRef.current?.click()}
          className={`flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-slate-300 p-6 text-center transition ${
            disabled || uploading ? 'opacity-50 cursor-not-allowed bg-slate-50' : 'cursor-pointer hover:border-amber-400 hover:bg-amber-50/20'
          }`}
        >
          {uploading ? (
            <div className="flex flex-col items-center gap-2 text-slate-600">
              <Loader2 className="h-8 w-8 animate-spin text-amber-500" />
              <p className="text-xs font-bold text-slate-800">{progressText || 'Uploading...'}</p>
              <p className="text-[11px] text-slate-400">Direct presigned PUT stream to MinIO storage</p>
            </div>
          ) : (
            <>
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-50 text-amber-600 mb-2">
                <FileUp className="h-6 w-6 stroke-[2.2]" />
              </div>
              <p className="text-xs font-bold text-slate-800">
                Click to attach official judgment or order PDF
              </p>
              <p className="text-[11px] text-slate-400 mt-1">
                Direct browser upload via presigned S3 URL (Max 50MB, PDF only)
              </p>
            </>
          )}
        </div>
      )}

      <input
        ref={fileInputRef}
        type="file"
        accept="application/pdf"
        className="hidden"
        onChange={handleFileChange}
        disabled={disabled || uploading}
      />
    </div>
  );
}
