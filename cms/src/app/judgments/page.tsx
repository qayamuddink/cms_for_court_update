'use client';

import React, { useState, useEffect, useRef } from 'react';
import apiClient, { getStoredTokens } from '@/lib/api/client';
import { useAuth } from '@/lib/auth/context';
import ProtectedRoute from '@/components/auth/ProtectedRoute';
import CMSLayout from '@/components/layout/CMSLayout';
import type { components } from '@/lib/api/schema';
import {
  BookOpen,
  Plus,
  Scale,
  Sparkles,
  ExternalLink,
  Loader2,
  CheckCircle2,
  AlertCircle,
  FileText,
  Download,
  Search,
  Upload,
  RefreshCw,
  FileCheck,
  ShieldAlert,
  Hash,
  Layers,
  ChevronRight,
  Trash2,
  Edit3,
  Calendar,
  Users,
  Award,
  FileQuestion,
  Eye,
  X,
} from 'lucide-react';

type JudgmentRecord = components['schemas']['OfficialJudgmentRecord'];
type StatuteInput = components['schemas']['StatuteCitedInput'];
type PageRecord = components['schemas']['JudgmentPageItem'];

const COMMON_COURTS = [
  { code: 'SC', name: 'Supreme Court of India' },
  { code: 'DHC', name: 'Delhi High Court' },
  { code: 'BHC', name: 'Bombay High Court' },
  { code: 'AHC', name: 'Allahabad High Court' },
  { code: 'KHC', name: 'Karnataka High Court' },
  { code: 'MHC', name: 'Madras High Court' },
  { code: 'CALHC', name: 'Calcutta High Court' },
  { code: 'PHHC', name: 'Punjab and Haryana High Court' },
  { code: 'GHC', name: 'Gauhati High Court' },
  { code: 'RHC', name: 'Rajasthan High Court' },
  { code: 'NCLAT', name: 'National Company Law Appellate Tribunal' },
  { code: 'NCLT', name: 'National Company Law Tribunal' },
  { code: 'ITAT', name: 'Income Tax Appellate Tribunal' },
];

export default function JudgmentsPage() {
  const { user } = useAuth();

  const [judgments, setJudgments] = useState<JudgmentRecord[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [liveCases, setLiveCases] = useState<any[]>([]);

  // Search and Filters
  const [searchQuery, setSearchQuery] = useState('');
  const [courtFilter, setCourtFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [accessFilter, setAccessFilter] = useState('');

  // Modals
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [editingJudgment, setEditingJudgment] = useState<JudgmentRecord | null>(null);
  const [pagesModalJudgment, setPagesModalJudgment] = useState<JudgmentRecord | null>(null);
  const [pagesList, setPagesList] = useState<PageRecord[]>([]);
  const [pagesLoading, setPagesLoading] = useState(false);
  const [publishModalJudgment, setPublishModalJudgment] = useState<JudgmentRecord | null>(null);
  const [brandPrefix, setBrandPrefix] = useState('CourtDesk');

  // Upload PDF Modal
  const [uploadModalJudgment, setUploadModalJudgment] = useState<JudgmentRecord | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const uploadFileInputRef = useRef<HTMLInputElement>(null);

  // Form state for Create / Edit
  const [formCaseTitle, setFormCaseTitle] = useState('');
  const [formCaseNumber, setFormCaseNumber] = useState('');
  const [formCourtCode, setFormCourtCode] = useState('SC');
  const [formCourtName, setFormCourtName] = useState('Supreme Court of India');
  const [formJudgmentDate, setFormJudgmentDate] = useState(() => new Date().toISOString().slice(0, 10));
  const [formDecisionDate, setFormDecisionDate] = useState('');
  const [formAppellant, setFormAppellant] = useState('');
  const [formRespondent, setFormRespondent] = useState('');
  const [formBench, setFormBench] = useState('');
  const [formAdvocates, setFormAdvocates] = useState('');
  const [formAccessLevel, setFormAccessLevel] = useState<'free' | 'premium'>('free');
  const [formCourtCaseId, setFormCourtCaseId] = useState('');
  const [formHeadnote, setFormHeadnote] = useState('');
  const [formSummary, setFormSummary] = useState('');
  const [formIssues, setFormIssues] = useState('');
  const [formHeld, setFormHeld] = useState('');
  const [formKeyTakeaways, setFormKeyTakeaways] = useState('');
  const [formSubjectTags, setFormSubjectTags] = useState('');
  const [formStatutes, setFormStatutes] = useState<StatuteInput[]>([
    { statute_name: '', sections_cited: [] },
  ]);
  const [formInitialPDF, setFormInitialPDF] = useState<File | null>(null);

  const [formSubmitting, setFormSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [successBanner, setSuccessBanner] = useState<string | null>(null);

  // Stats calculation
  const stats = {
    total: judgments.length,
    published: judgments.filter((j) => j.status === 'published' && j.citation).length,
    ocrExtracted: judgments.filter(
      (j) => j.extraction_status === 'completed' || j.extraction_status === 'ocr_completed'
    ).length,
    premium: judgments.filter((j) => j.access_level === 'premium').length,
  };

  const fetchJudgments = async () => {
    setLoading(true);
    try {
      const queryParams: any = { limit: 100 };
      if (statusFilter) queryParams.status = statusFilter;
      if (courtFilter) queryParams.court_code = courtFilter;
      if (searchQuery.trim()) queryParams.search = searchQuery.trim();

      const { data, response } = await apiClient.GET('/v1/cms/judgments', {
        params: { query: queryParams },
      });

      if (response.ok && data) {
        setJudgments((data as any).judgments || []);
        setTotalCount((data as any).total || 0);
      }
    } catch (err) {
      // Fallback
    } finally {
      setLoading(false);
    }
  };

  const fetchCases = async () => {
    try {
      const { data } = await apiClient.GET('/v1/cases/live');
      const items = (data as any)?.cases || (Array.isArray(data) ? data : []);
      setLiveCases(items);
    } catch {
      // Ignore
    }
  };

  useEffect(() => {
    fetchJudgments();
    fetchCases();
  }, [courtFilter, statusFilter]);

  const handleCourtChange = (code: string) => {
    setFormCourtCode(code);
    const match = COMMON_COURTS.find((c) => c.code === code);
    if (match) {
      setFormCourtName(match.name);
    }
  };

  const handleOpenCreateModal = () => {
    setEditingJudgment(null);
    setFormCaseTitle('');
    setFormCaseNumber('');
    setFormCourtCode('SC');
    setFormCourtName('Supreme Court of India');
    setFormJudgmentDate(new Date().toISOString().slice(0, 10));
    setFormDecisionDate('');
    setFormAppellant('');
    setFormRespondent('');
    setFormBench('');
    setFormAdvocates('');
    setFormAccessLevel('free');
    setFormCourtCaseId('');
    setFormHeadnote('');
    setFormSummary('');
    setFormIssues('');
    setFormHeld('');
    setFormKeyTakeaways('');
    setFormSubjectTags('');
    setFormStatutes([{ statute_name: '', sections_cited: [] }]);
    setFormInitialPDF(null);
    setFormError(null);
    setCreateModalOpen(true);
  };

  const handleOpenEditModal = async (j: JudgmentRecord) => {
    setEditingJudgment(j);
    setFormCaseTitle(j.case_title || '');
    setFormCaseNumber(j.case_number || '');
    setFormCourtCode(j.court_code || 'SC');
    setFormCourtName(j.court || 'Supreme Court of India');
    setFormJudgmentDate(j.judgment_date ? j.judgment_date.slice(0, 10) : '');
    setFormDecisionDate(j.decision_date ? j.decision_date.slice(0, 10) : '');
    setFormAppellant(j.appellant || '');
    setFormRespondent(j.respondent || '');
    setFormBench((j.bench || []).join(', '));
    setFormAdvocates((j.advocates || []).join(', '));
    setFormAccessLevel(j.access_level || 'free');
    setFormCourtCaseId(j.court_case_id || '');
    setFormHeadnote(j.headnote || '');
    setFormSummary(j.summary || '');
    setFormIssues(j.issues || '');
    setFormHeld(j.held || '');
    setFormKeyTakeaways((j.key_takeaways || []).join('\n'));
    setFormSubjectTags((j.subject_tags || []).join(', '));
    setFormInitialPDF(null);
    setFormError(null);

    // Fetch existing statutes
    try {
      const { data } = await apiClient.GET('/v1/cms/judgments/{id}', {
        params: { path: { id: j.id } },
      });
      if (data && (data as any).statutes) {
        const mapped = (data as any).statutes.map((s: any) => ({
          statute_name: s.statute_name,
          sections_cited: s.sections_cited || [],
        }));
        setFormStatutes(mapped.length > 0 ? mapped : [{ statute_name: '', sections_cited: [] }]);
      }
    } catch {
      setFormStatutes([{ statute_name: '', sections_cited: [] }]);
    }

    setCreateModalOpen(true);
  };

  const handleAddStatuteRow = () => {
    setFormStatutes([...formStatutes, { statute_name: '', sections_cited: [] }]);
  };

  const handleRemoveStatuteRow = (index: number) => {
    setFormStatutes(formStatutes.filter((_, i) => i !== index));
  };

  const handleStatuteNameChange = (index: number, val: string) => {
    const updated = [...formStatutes];
    updated[index].statute_name = val;
    setFormStatutes(updated);
  };

  const handleStatuteSectionsChange = (index: number, val: string) => {
    const updated = [...formStatutes];
    updated[index].sections_cited = val
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
    setFormStatutes(updated);
  };

  const handleSaveJudgment = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formCaseTitle.trim() || !formCaseNumber.trim() || !formSummary.trim()) {
      setFormError('Case Title, Case Number, and Summary are required.');
      return;
    }
    if (!formAppellant.trim() || !formRespondent.trim()) {
      setFormError('Both Appellant and Respondent party names are required.');
      return;
    }

    setFormSubmitting(true);
    setFormError(null);

    const parseList = (str: string) =>
      str
        .split(/[,\n]/)
        .map((s) => s.trim())
        .filter(Boolean);

    const cleanStatutes = formStatutes.filter((s) => s.statute_name.trim());

    const payload: any = {
      case_title: formCaseTitle.trim(),
      case_number: formCaseNumber.trim(),
      court: formCourtName.trim(),
      court_code: formCourtCode.trim().toUpperCase(),
      bench: parseList(formBench),
      judgment_date: formJudgmentDate,
      decision_date: formDecisionDate || undefined,
      appellant: formAppellant.trim(),
      respondent: formRespondent.trim(),
      advocates: parseList(formAdvocates),
      subject_tags: parseList(formSubjectTags),
      issues: formIssues.trim() || undefined,
      held: formHeld.trim() || undefined,
      headnote: formHeadnote.trim() || undefined,
      summary: formSummary.trim(),
      key_takeaways: formKeyTakeaways
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean),
      statutes: cleanStatutes,
      court_case_id: formCourtCaseId || undefined,
      access_level: formAccessLevel,
    };

    try {
      let savedJudgmentId = editingJudgment?.id;

      if (editingJudgment) {
        // Update existing
        const { data, error, response } = await apiClient.PUT('/v1/cms/judgments/{id}', {
          params: { path: { id: editingJudgment.id } },
          body: payload,
        });
        if (!response.ok || !data) {
          throw new Error((error as any)?.message || 'Failed to update judgment');
        }
        setSuccessBanner(`Updated judgment: "${payload.case_title}"`);
      } else {
        // Create new draft
        const { data, error, response } = await apiClient.POST('/v1/cms/judgments', {
          body: payload,
        });
        if (!response.ok || !data) {
          throw new Error((error as any)?.message || 'Failed to create judgment draft');
        }
        savedJudgmentId = (data as any).judgment?.id || (data as any).id;
        setSuccessBanner(`Created official judgment draft: "${payload.case_title}"`);

        // If an initial PDF was selected, upload it immediately
        if (formInitialPDF && savedJudgmentId) {
          await uploadPDFToJudgment(savedJudgmentId, formInitialPDF);
        }
      }

      setCreateModalOpen(false);
      fetchJudgments();
    } catch (err: any) {
      setFormError(err.message || 'Error saving judgment');
    } finally {
      setFormSubmitting(false);
    }
  };

  // Upload PDF directly to /v1/cms/judgments/{id}/pdf
  const uploadPDFToJudgment = async (judgmentId: string, file: File) => {
    // Client-side magic bytes validation
    const headerBytes = new Uint8Array(await file.slice(0, 5).arrayBuffer());
    const headerStr = Array.from(headerBytes)
      .map((b) => String.fromCharCode(b))
      .join('');
    if (!headerStr.startsWith('%PDF-')) {
      throw new Error('Selected file does not have valid %PDF- magic bytes.');
    }

    const { accessToken } = getStoredTokens();
    const formData = new FormData();
    formData.append('file', file);

    const res = await fetch(`http://localhost:8080/v1/cms/judgments/${judgmentId}/pdf`, {
      method: 'POST',
      headers: {
        ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      },
      body: formData,
    });

    if (!res.ok) {
      const errJson = await res.json().catch(() => ({}));
      if (res.status === 409) {
        throw new Error('Duplicate PDF detected: This exact PDF checksum is already attached to another judgment.');
      }
      if (res.status === 422) {
        throw new Error('Virus scanner rejected file: Threat signature detected in PDF.');
      }
      throw new Error(errJson.message || `Upload failed with HTTP ${res.status}`);
    }
  };

  const handleUploadSubmit = async () => {
    if (!uploadModalJudgment || !selectedFile) return;

    setUploading(true);
    setUploadError(null);

    try {
      await uploadPDFToJudgment(uploadModalJudgment.id, selectedFile);
      setSuccessBanner(`Official PDF attached to "${uploadModalJudgment.case_title}". Background OCR extraction queued.`);
      setUploadModalJudgment(null);
      setSelectedFile(null);
      fetchJudgments();
    } catch (err: any) {
      setUploadError(err.message || 'Failed to attach official PDF');
    } finally {
      setUploading(false);
    }
  };

  const handlePublishClick = async () => {
    if (!publishModalJudgment) return;

    setFormSubmitting(true);
    try {
      const { data, error, response } = await apiClient.POST('/v1/cms/judgments/{id}/publish', {
        params: { path: { id: publishModalJudgment.id } },
        body: { brand_prefix: brandPrefix.trim() || 'CourtDesk' },
      });

      if (!response.ok || !data) {
        throw new Error((error as any)?.message || 'Failed to publish judgment');
      }

      const assignedCitation = (data as any).citation;
      setSuccessBanner(
        `Judgment officially published! Allocated sequential citation: ${assignedCitation}`
      );
      setPublishModalJudgment(null);
      fetchJudgments();
    } catch (err: any) {
      alert(`Publish error: ${err.message}`);
    } finally {
      setFormSubmitting(false);
    }
  };

  const handleDownloadPDF = async (judgmentId: string) => {
    try {
      const { data, error, response } = await apiClient.GET('/v1/judgments/{id}/download', {
        params: { path: { id: judgmentId } },
      });

      if (!response.ok || !data) {
        throw new Error((error as any)?.message || 'Failed to obtain download URL');
      }

      const downloadURL = (data as any).download_url;
      if (downloadURL) {
        window.open(downloadURL, '_blank');
      }
    } catch (err: any) {
      alert(`Download error: ${err.message}`);
    }
  };

  const handleOpenPagesModal = async (j: JudgmentRecord) => {
    setPagesModalJudgment(j);
    setPagesLoading(true);
    try {
      const { data, response } = await apiClient.GET('/v1/judgments/{id}/pages', {
        params: { path: { id: j.id } },
      });
      if (response.ok && data) {
        setPagesList((data as any).pages || []);
      } else {
        setPagesList([]);
      }
    } catch {
      setPagesList([]);
    } finally {
      setPagesLoading(false);
    }
  };

  const filteredJudgments = judgments.filter((j) => {
    if (accessFilter && j.access_level !== accessFilter) return false;
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase();
    return (
      j.case_title.toLowerCase().includes(q) ||
      j.case_number.toLowerCase().includes(q) ||
      (j.citation && j.citation.toLowerCase().includes(q)) ||
      j.court.toLowerCase().includes(q) ||
      j.court_code.toLowerCase().includes(q) ||
      (j.bench && j.bench.some((b) => b.toLowerCase().includes(q))) ||
      (j.appellant && j.appellant.toLowerCase().includes(q)) ||
      (j.respondent && j.respondent.toLowerCase().includes(q))
    );
  });

  const getExtractionStatusBadge = (status: string) => {
    switch (status) {
      case 'completed':
        return (
          <span className="inline-flex items-center gap-1 text-[11px] font-extrabold text-emerald-800 bg-emerald-50 border border-emerald-200 px-2 py-0.5 rounded-md">
            <CheckCircle2 className="h-3 w-3 text-emerald-600" />
            Text Extracted
          </span>
        );
      case 'ocr_completed':
        return (
          <span className="inline-flex items-center gap-1 text-[11px] font-extrabold text-blue-800 bg-blue-50 border border-blue-200 px-2 py-0.5 rounded-md">
            <Sparkles className="h-3 w-3 text-blue-600" />
            OCR Scanned
          </span>
        );
      case 'processing':
        return (
          <span className="inline-flex items-center gap-1 text-[11px] font-extrabold text-amber-800 bg-amber-50 border border-amber-200 px-2 py-0.5 rounded-md">
            <Loader2 className="h-3 w-3 animate-spin text-amber-600" />
            Extracting...
          </span>
        );
      case 'failed':
        return (
          <span className="inline-flex items-center gap-1 text-[11px] font-extrabold text-red-800 bg-red-50 border border-red-200 px-2 py-0.5 rounded-md">
            <AlertCircle className="h-3 w-3 text-red-600" />
            Extraction Failed
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 text-[11px] font-semibold text-slate-500 bg-slate-100 px-2 py-0.5 rounded-md">
            Pending Queue
          </span>
        );
    }
  };

  return (
    <ProtectedRoute allowedRoles={['editor', 'admin']}>
      <CMSLayout
        title="Official Judgments, Citations & Headnotes Desk"
        badge={`${judgments.length} Decisions`}
      >
        {/* Success Banner */}
        {successBanner && (
          <div className="mb-6 flex items-center justify-between rounded-2xl bg-emerald-50 border border-emerald-200 p-4 text-xs font-bold text-emerald-900 shadow-sm animate-in fade-in">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="h-5 w-5 text-emerald-600 shrink-0" />
              <span>{successBanner}</span>
            </div>
            <button
              type="button"
              onClick={() => setSuccessBanner(null)}
              className="text-emerald-700 hover:text-emerald-900 font-semibold underline text-xs ml-4"
            >
              Dismiss
            </button>
          </div>
        )}

        {/* Top Metric Cards */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
          <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
            <div className="flex items-center justify-between text-slate-500 mb-1">
              <span className="text-xs font-bold uppercase tracking-wider">Total Decisions</span>
              <Scale className="h-4 w-4 text-slate-400" />
            </div>
            <p className="text-2xl font-black text-slate-900">{stats.total}</p>
          </div>

          <div className="rounded-2xl border border-amber-200 bg-gradient-to-br from-amber-50 to-amber-100/50 p-4 shadow-sm">
            <div className="flex items-center justify-between text-amber-800 mb-1">
              <span className="text-xs font-extrabold uppercase tracking-wider">Citations Allocated</span>
              <Award className="h-4 w-4 text-amber-600" />
            </div>
            <p className="text-2xl font-black text-amber-950">{stats.published}</p>
            <p className="text-[10px] text-amber-800 font-medium mt-0.5">Atomic sequential numbers</p>
          </div>

          <div className="rounded-2xl border border-blue-200 bg-gradient-to-br from-blue-50 to-blue-100/50 p-4 shadow-sm">
            <div className="flex items-center justify-between text-blue-800 mb-1">
              <span className="text-xs font-extrabold uppercase tracking-wider">OCR &amp; Extracted</span>
              <Layers className="h-4 w-4 text-blue-600" />
            </div>
            <p className="text-2xl font-black text-blue-950">{stats.ocrExtracted}</p>
            <p className="text-[10px] text-blue-800 font-medium mt-0.5">Full-text indexed pages</p>
          </div>

          <div className="rounded-2xl border border-purple-200 bg-gradient-to-br from-purple-50 to-purple-100/50 p-4 shadow-sm">
            <div className="flex items-center justify-between text-purple-800 mb-1">
              <span className="text-xs font-extrabold uppercase tracking-wider">Premium Access</span>
              <Sparkles className="h-4 w-4 text-purple-600" />
            </div>
            <p className="text-2xl font-black text-purple-950">{stats.premium}</p>
            <p className="text-[10px] text-purple-800 font-medium mt-0.5">Entitlement gated PDFs</p>
          </div>
        </div>

        {/* Filter Controls Bar */}
        <div className="rounded-2xl border border-slate-200 bg-white p-4 mb-6 shadow-sm flex flex-col md:flex-row items-stretch md:items-center justify-between gap-4">
          <div className="flex flex-1 flex-wrap items-center gap-3">
            {/* Search */}
            <div className="relative flex-1 min-w-[240px]">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400" />
              <input
                type="text"
                placeholder="Search by case title, citation, bench, or case number..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full rounded-xl border border-slate-200 bg-slate-50/50 pl-9 pr-3 py-2 text-xs font-medium text-slate-800 placeholder:text-slate-400 focus:border-amber-400 focus:bg-white focus:outline-none"
              />
            </div>

            {/* Court Filter */}
            <select
              value={courtFilter}
              onChange={(e) => setCourtFilter(e.target.value)}
              className="rounded-xl border border-slate-200 bg-slate-50/50 px-3 py-2 text-xs font-bold text-slate-700 focus:border-amber-400 focus:outline-none"
            >
              <option value="">All Courts</option>
              {COMMON_COURTS.map((c) => (
                <option key={c.code} value={c.code}>
                  {c.code} - {c.name}
                </option>
              ))}
            </select>

            {/* Status Filter */}
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="rounded-xl border border-slate-200 bg-slate-50/50 px-3 py-2 text-xs font-bold text-slate-700 focus:border-amber-400 focus:outline-none"
            >
              <option value="">All Statuses</option>
              <option value="draft">Draft (Unpublished)</option>
              <option value="published">Published with Citation</option>
            </select>

            {/* Access Filter */}
            <select
              value={accessFilter}
              onChange={(e) => setAccessFilter(e.target.value)}
              className="rounded-xl border border-slate-200 bg-slate-50/50 px-3 py-2 text-xs font-bold text-slate-700 focus:border-amber-400 focus:outline-none"
            >
              <option value="">All Access</option>
              <option value="free">Free Access</option>
              <option value="premium">Premium Only</option>
            </select>

            <button
              type="button"
              onClick={fetchJudgments}
              className="rounded-xl border border-slate-200 p-2 text-slate-500 hover:bg-slate-100 hover:text-slate-800 transition"
              title="Refresh judgments list"
            >
              <RefreshCw className="h-4 w-4" />
            </button>
          </div>

          {/* New Judgment Button */}
          <button
            type="button"
            onClick={handleOpenCreateModal}
            className="inline-flex items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 px-5 py-2.5 text-xs font-black text-slate-950 shadow-md shadow-amber-500/20 hover:from-amber-400 hover:to-amber-500 transition shrink-0"
          >
            <Plus className="h-4 w-4 stroke-[3]" />
            New Official Judgment
          </button>
        </div>

        {/* Judgments List */}
        {loading ? (
          <div className="py-24 text-center text-slate-400">
            <Loader2 className="h-8 w-8 animate-spin mx-auto mb-3 text-amber-500" />
            <p className="text-xs font-semibold">Loading official judgments database...</p>
          </div>
        ) : filteredJudgments.length === 0 ? (
          <div className="rounded-2xl bg-white border border-slate-200 p-12 text-center shadow-sm">
            <BookOpen className="h-12 w-12 text-slate-300 mx-auto mb-3" />
            <p className="text-base font-bold text-slate-700">No judgments found</p>
            <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
              Click &quot;New Official Judgment&quot; to publish a judicial decision with structured headnotes and official citation allocation.
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            {filteredJudgments.map((j) => (
              <div
                key={j.id}
                className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm hover:border-slate-300 transition"
              >
                {/* Header row: Court, Citation, Status, Access */}
                <div className="flex flex-wrap items-center justify-between gap-2 pb-3 border-b border-slate-100">
                  <div className="flex flex-wrap items-center gap-2">
                    {/* Court Badge */}
                    <span className="inline-flex items-center gap-1.5 rounded-lg bg-blue-50 border border-blue-200 px-2.5 py-1 text-xs font-extrabold text-blue-900">
                      <Scale className="h-3.5 w-3.5 text-blue-700" />
                      {j.court} ({j.court_code})
                    </span>

                    {/* Official Citation or Draft Pill */}
                    {j.citation ? (
                      <span className="inline-flex items-center gap-1.5 rounded-lg bg-gradient-to-r from-amber-100 to-amber-200/80 border border-amber-300 px-3 py-1 font-mono text-xs font-black text-amber-950 shadow-xs">
                        <Award className="h-3.5 w-3.5 text-amber-800" />
                        {j.citation}
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 rounded-lg bg-slate-100 border border-slate-200 px-2.5 py-1 text-xs font-bold text-slate-600">
                        DRAFT (Unpublished)
                      </span>
                    )}

                    {/* Access Level */}
                    {j.access_level === 'premium' ? (
                      <span className="inline-flex items-center gap-1 rounded-lg bg-purple-50 border border-purple-200 px-2 py-0.5 text-[11px] font-extrabold text-purple-900">
                        <Sparkles className="h-3 w-3 text-purple-600" />
                        Premium
                      </span>
                    ) : (
                      <span className="inline-flex items-center rounded-lg bg-slate-100 px-2 py-0.5 text-[11px] font-semibold text-slate-600">
                        Free Access
                      </span>
                    )}

                    {/* Extraction Status */}
                    {getExtractionStatusBadge(j.extraction_status)}
                  </div>

                  {/* Date */}
                  <div className="flex items-center gap-1.5 text-xs text-slate-500 font-semibold">
                    <Calendar className="h-3.5 w-3.5 text-slate-400" />
                    <span>
                      {j.judgment_date
                        ? new Date(j.judgment_date).toLocaleDateString('en-IN', {
                            day: 'numeric',
                            month: 'short',
                            year: 'numeric',
                          })
                        : 'No date'}
                    </span>
                  </div>
                </div>

                {/* Case Title & Case Number */}
                <div className="mt-3">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h3 className="text-base font-extrabold text-slate-900 hover:text-amber-600 transition">
                        {j.case_title}
                      </h3>
                      <p className="text-xs font-mono font-bold text-slate-500 mt-0.5">
                        {j.case_number}
                      </p>
                    </div>

                    <div className="flex items-center gap-2 shrink-0">
                      <button
                        type="button"
                        onClick={() => handleOpenEditModal(j)}
                        className="rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-700 px-3 py-1.5 text-xs font-bold transition flex items-center gap-1"
                      >
                        <Edit3 className="h-3.5 w-3.5" />
                        Edit
                      </button>

                      <a
                        href={
                          j.citation
                            ? `http://localhost:8080/v1/citations/${encodeURIComponent(j.citation)}`
                            : `http://localhost:8080/v1/judgments/${j.id}`
                        }
                        target="_blank"
                        rel="noreferrer"
                        className="rounded-lg border border-slate-200 hover:bg-slate-50 text-slate-600 px-2.5 py-1.5 text-xs font-semibold flex items-center gap-1 transition"
                        title="View Public API Output"
                      >
                        <ExternalLink className="h-3.5 w-3.5" />
                        API
                      </a>
                    </div>
                  </div>

                  {/* Parties & Bench */}
                  <div className="mt-2.5 grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs text-slate-600 bg-slate-50/60 rounded-xl p-3 border border-slate-100">
                    <div>
                      <span className="font-bold text-slate-800">Parties: </span>
                      <span>{j.appellant}</span>
                      <span className="text-slate-400 font-bold mx-1.5">v.</span>
                      <span>{j.respondent}</span>
                    </div>

                    {j.bench && j.bench.length > 0 && (
                      <div className="truncate">
                        <span className="font-bold text-slate-800">Bench: </span>
                        <span>{j.bench.join(', ')}</span>
                      </div>
                    )}
                  </div>

                  {/* Headnote Preview */}
                  {j.headnote && (
                    <div className="mt-2.5 rounded-xl border border-amber-200/80 bg-amber-50/40 p-3 text-xs leading-relaxed text-slate-800 font-serif italic">
                      <span className="font-sans not-italic font-bold text-amber-900 block mb-0.5 text-[11px] uppercase tracking-wider">
                        Headnote (Ratio Decidendi)
                      </span>
                      {j.headnote}
                    </div>
                  )}

                  {/* Summary */}
                  <p className="mt-2 text-xs leading-relaxed text-slate-600 line-clamp-2">
                    {j.summary}
                  </p>

                  {/* Subject Tags & Key Takeaways */}
                  {((j.subject_tags && j.subject_tags.length > 0) ||
                    (j.key_takeaways && j.key_takeaways.length > 0)) && (
                    <div className="mt-2.5 flex flex-wrap items-center gap-1.5">
                      {j.subject_tags?.map((tag) => (
                        <span
                          key={tag}
                          className="rounded-md bg-slate-100 border border-slate-200 px-2 py-0.5 text-[10px] font-bold text-slate-700"
                        >
                          #{tag}
                        </span>
                      ))}
                      {j.key_takeaways?.slice(0, 2).map((takeaway, i) => (
                        <span
                          key={i}
                          className="rounded-md bg-emerald-50 border border-emerald-200 px-2 py-0.5 text-[10px] font-semibold text-emerald-800"
                        >
                          ✓ {takeaway}
                        </span>
                      ))}
                    </div>
                  )}
                </div>

                {/* Bottom Footer: PDF Attachment, Actions */}
                <div className="mt-4 pt-3 border-t border-slate-100 flex flex-wrap items-center justify-between gap-3 text-xs">
                  {/* Left: PDF Info */}
                  <div className="flex items-center gap-2">
                    {j.pdf_storage_key ? (
                      <div className="flex items-center gap-2">
                        <span className="inline-flex items-center gap-1 font-bold text-emerald-700 bg-emerald-50 border border-emerald-200 px-2.5 py-1 rounded-lg">
                          <FileCheck className="h-3.5 w-3.5 text-emerald-600" />
                          <span>
                            {j.pdf_filename || 'Official Judgment PDF'}
                          </span>
                          {j.pdf_page_count ? (
                            <span className="font-semibold text-emerald-600 text-[11px] ml-1">
                              ({j.pdf_page_count} pages)
                            </span>
                          ) : null}
                        </span>

                        {j.pdf_size_bytes && (
                          <span className="text-[11px] text-slate-400 font-mono">
                            {(j.pdf_size_bytes / (1024 * 1024)).toFixed(2)} MB
                          </span>
                        )}
                      </div>
                    ) : (
                      <span className="text-slate-400 flex items-center gap-1 text-[11px] font-semibold">
                        <AlertCircle className="h-3.5 w-3.5 text-amber-500" />
                        No official PDF attached yet
                      </span>
                    )}
                  </div>

                  {/* Right: Actions */}
                  <div className="flex flex-wrap items-center gap-2">
                    {/* View Extracted Pages Button */}
                    {j.pdf_storage_key && (
                      <button
                        type="button"
                        onClick={() => handleOpenPagesModal(j)}
                        className="rounded-lg border border-slate-200 bg-white hover:bg-slate-50 text-slate-700 px-2.5 py-1 text-xs font-bold transition flex items-center gap-1"
                      >
                        <Eye className="h-3.5 w-3.5 text-slate-500" />
                        Extracted Pages
                      </button>
                    )}

                    {/* Download PDF button */}
                    {j.pdf_storage_key && (
                      <button
                        type="button"
                        onClick={() => handleDownloadPDF(j.id)}
                        className="rounded-lg border border-slate-200 bg-white hover:bg-slate-50 text-slate-700 px-2.5 py-1 text-xs font-bold transition flex items-center gap-1"
                      >
                        <Download className="h-3.5 w-3.5 text-slate-500" />
                        Download PDF
                      </button>
                    )}

                    {/* Attach or Replace PDF */}
                    <button
                      type="button"
                      onClick={() => {
                        setUploadModalJudgment(j);
                        setSelectedFile(null);
                        setUploadError(null);
                      }}
                      className="rounded-lg border border-slate-200 bg-white hover:bg-slate-50 text-slate-700 px-2.5 py-1 text-xs font-bold transition flex items-center gap-1"
                    >
                      <Upload className="h-3.5 w-3.5 text-slate-500" />
                      {j.pdf_storage_key ? 'Replace PDF' : 'Attach PDF'}
                    </button>

                    {/* Publish & Allocate Citation button */}
                    {j.status === 'draft' && (
                      <button
                        type="button"
                        onClick={() => {
                          setPublishModalJudgment(j);
                          setBrandPrefix('CourtDesk');
                        }}
                        disabled={!j.pdf_storage_key}
                        className="rounded-lg bg-gradient-to-r from-amber-500 to-amber-600 hover:from-amber-400 hover:to-amber-500 text-slate-950 px-3 py-1 font-black text-xs shadow-xs transition disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-1"
                        title={
                          !j.pdf_storage_key
                            ? 'Attach an official PDF before publishing'
                            : 'Atomically allocate citation and publish'
                        }
                      >
                        <Award className="h-3.5 w-3.5 stroke-[2.5]" />
                        Publish &amp; Allocate Citation
                      </button>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Modal: Create or Edit Judgment */}
        {createModalOpen && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 backdrop-blur-sm p-4 overflow-y-auto animate-in fade-in">
            <div className="w-full max-w-3xl rounded-2xl bg-white p-6 shadow-2xl border border-slate-200 my-8 max-h-[90vh] overflow-y-auto">
              <div className="flex items-center justify-between pb-3 border-b border-slate-100 mb-5">
                <div className="flex items-center gap-2">
                  <Scale className="h-5 w-5 text-amber-500" />
                  <h3 className="text-base font-black text-slate-900">
                    {editingJudgment ? 'Edit Official Judgment' : 'Create Official Judgment Draft'}
                  </h3>
                </div>
                <button
                  type="button"
                  onClick={() => setCreateModalOpen(false)}
                  className="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-800"
                >
                  <X className="h-5 w-5" />
                </button>
              </div>

              {formError && (
                <div className="mb-4 flex items-center gap-2 rounded-xl bg-red-50 border border-red-200 p-3 text-xs font-bold text-red-700">
                  <AlertCircle className="h-4 w-4 shrink-0 text-red-600" />
                  <span>{formError}</span>
                </div>
              )}

              <form onSubmit={handleSaveJudgment} className="space-y-4">
                {/* Court and Case Number */}
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Court Selection *
                    </label>
                    <select
                      value={formCourtCode}
                      onChange={(e) => handleCourtChange(e.target.value)}
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-bold text-slate-800 focus:border-amber-400 focus:outline-none"
                    >
                      {COMMON_COURTS.map((c) => (
                        <option key={c.code} value={c.code}>
                          {c.code} - {c.name}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Court Name *
                    </label>
                    <input
                      type="text"
                      required
                      value={formCourtName}
                      onChange={(e) => setFormCourtName(e.target.value)}
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-medium text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Case Number *
                    </label>
                    <input
                      type="text"
                      required
                      value={formCaseNumber}
                      onChange={(e) => setFormCaseNumber(e.target.value)}
                      placeholder="e.g. Civil Appeal No. 1243 of 2026"
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-mono font-bold text-slate-900 focus:border-amber-400 focus:outline-none"
                    />
                  </div>
                </div>

                {/* Case Title */}
                <div>
                  <label className="block text-xs font-bold text-slate-700 mb-1">
                    Case Title *
                  </label>
                  <input
                    type="text"
                    required
                    value={formCaseTitle}
                    onChange={(e) => setFormCaseTitle(e.target.value)}
                    placeholder="e.g. State of Gujarat v. Union of India &amp; Ors."
                    className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2.5 text-xs font-extrabold text-slate-900 focus:border-amber-400 focus:outline-none"
                  />
                </div>

                {/* Parties: Appellant & Respondent */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Appellant / Petitioner *
                    </label>
                    <input
                      type="text"
                      required
                      value={formAppellant}
                      onChange={(e) => setFormAppellant(e.target.value)}
                      placeholder="e.g. State of Gujarat"
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-medium text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Respondent *
                    </label>
                    <input
                      type="text"
                      required
                      value={formRespondent}
                      onChange={(e) => setFormRespondent(e.target.value)}
                      placeholder="e.g. Union of India"
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-medium text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>
                </div>

                {/* Dates & Bench */}
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Judgment Date *
                    </label>
                    <input
                      type="date"
                      required
                      value={formJudgmentDate}
                      onChange={(e) => setFormJudgmentDate(e.target.value)}
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-medium text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Decision Date (Optional)
                    </label>
                    <input
                      type="date"
                      value={formDecisionDate}
                      onChange={(e) => setFormDecisionDate(e.target.value)}
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-medium text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Access Tier
                    </label>
                    <select
                      value={formAccessLevel}
                      onChange={(e) => setFormAccessLevel(e.target.value as any)}
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-bold text-slate-800 focus:border-amber-400 focus:outline-none"
                    >
                      <option value="free">Free Public Access</option>
                      <option value="premium">Premium Paywalled</option>
                    </select>
                  </div>
                </div>

                {/* Bench & Advocates */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Bench / Judges (comma-separated)
                    </label>
                    <input
                      type="text"
                      value={formBench}
                      onChange={(e) => setFormBench(e.target.value)}
                      placeholder="e.g. D.Y. Chandrachud CJI, Sanjiv Khanna J."
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Advocates / Counsel (comma-separated)
                    </label>
                    <input
                      type="text"
                      value={formAdvocates}
                      onChange={(e) => setFormAdvocates(e.target.value)}
                      placeholder="e.g. Tushar Mehta SG, Kapil Sibal Sr. Adv."
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>
                </div>

                {/* Headnote */}
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <label className="block text-xs font-bold uppercase tracking-wider text-amber-900">
                      Headnote &amp; Ratio Decidendi
                    </label>
                    <span className="text-[11px] text-slate-400">Core legal principle held</span>
                  </div>
                  <textarea
                    rows={3}
                    value={formHeadnote}
                    onChange={(e) => setFormHeadnote(e.target.value)}
                    placeholder="Summarize the core point of law determined by the court..."
                    className="w-full rounded-xl border border-amber-200 bg-amber-50/30 p-2.5 text-xs font-serif leading-relaxed text-slate-900 focus:border-amber-400 focus:bg-white focus:outline-none"
                  />
                </div>

                {/* Summary */}
                <div>
                  <label className="block text-xs font-bold text-slate-700 mb-1">
                    Judgment Summary *
                  </label>
                  <textarea
                    rows={3}
                    required
                    value={formSummary}
                    onChange={(e) => setFormSummary(e.target.value)}
                    placeholder="Comprehensive overview of facts, procedural history, and court deliberations..."
                    className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2.5 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                  />
                </div>

                {/* Issues & Held */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Legal Issues Formulated
                    </label>
                    <textarea
                      rows={2}
                      value={formIssues}
                      onChange={(e) => setFormIssues(e.target.value)}
                      placeholder="e.g. Whether Article 19(1)(a) includes..."
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Held / Operative Order
                    </label>
                    <textarea
                      rows={2}
                      value={formHeld}
                      onChange={(e) => setFormHeld(e.target.value)}
                      placeholder="e.g. Appeal allowed; directions issued..."
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>
                </div>

                {/* Key Takeaways & Subject Tags */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Key Takeaways (one per line)
                    </label>
                    <textarea
                      rows={2}
                      value={formKeyTakeaways}
                      onChange={(e) => setFormKeyTakeaways(e.target.value)}
                      placeholder="Bullet point takeaways..."
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-bold text-slate-700 mb-1">
                      Subject Tags (comma-separated)
                    </label>
                    <input
                      type="text"
                      value={formSubjectTags}
                      onChange={(e) => setFormSubjectTags(e.target.value)}
                      placeholder="e.g. Constitutional Law, GST, RTI"
                      className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                    />
                  </div>
                </div>

                {/* Structured Statutes Cited */}
                <div className="rounded-xl border border-slate-200 bg-slate-50/40 p-4">
                  <div className="flex items-center justify-between mb-3">
                    <span className="text-xs font-bold uppercase tracking-wider text-slate-800">
                      Structured Statutes Cited
                    </span>
                    <button
                      type="button"
                      onClick={handleAddStatuteRow}
                      className="text-xs font-bold text-amber-700 hover:text-amber-800 bg-amber-50 border border-amber-200 px-2.5 py-1 rounded-lg transition"
                    >
                      + Add Statute
                    </button>
                  </div>

                  <div className="space-y-2">
                    {formStatutes.map((st, idx) => (
                      <div key={idx} className="flex items-center gap-2">
                        <input
                          type="text"
                          placeholder="Statute name (e.g. Constitution of India)"
                          value={st.statute_name}
                          onChange={(e) => handleStatuteNameChange(idx, e.target.value)}
                          className="flex-1 rounded-xl border border-slate-200 bg-white p-2 text-xs font-semibold text-slate-800 focus:border-amber-400 focus:outline-none"
                        />
                        <input
                          type="text"
                          placeholder="Sections (comma-separated: Art. 19, Art. 21)"
                          value={(st.sections_cited || []).join(', ')}
                          onChange={(e) => handleStatuteSectionsChange(idx, e.target.value)}
                          className="flex-1 rounded-xl border border-slate-200 bg-white p-2 text-xs text-slate-800 focus:border-amber-400 focus:outline-none"
                        />
                        {formStatutes.length > 1 && (
                          <button
                            type="button"
                            onClick={() => handleRemoveStatuteRow(idx)}
                            className="p-1.5 text-slate-400 hover:text-red-600 rounded-lg"
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        )}
                      </div>
                    ))}
                  </div>
                </div>

                {/* PDF File Picker (For Create Draft) */}
                {!editingJudgment && (
                  <div className="rounded-xl border border-slate-200 bg-slate-50/40 p-4">
                    <label className="block text-xs font-bold uppercase tracking-wider text-slate-800 mb-1">
                      Attach Official Judgment PDF (Optional at Draft Stage)
                    </label>
                    <p className="text-[11px] text-slate-500 mb-2">
                      Direct upload with %PDF- magic bytes validation, duplicate checksum detection, and OCR extraction.
                    </p>
                    <input
                      type="file"
                      accept="application/pdf"
                      onChange={(e) => {
                        const file = e.target.files?.[0];
                        if (file) setFormInitialPDF(file);
                      }}
                      className="text-xs text-slate-700 file:mr-3 file:py-1.5 file:px-3 file:rounded-xl file:border-0 file:text-xs file:font-bold file:bg-amber-100 file:text-amber-900 hover:file:bg-amber-200"
                    />
                    {formInitialPDF && (
                      <p className="text-xs font-semibold text-emerald-700 mt-2">
                        ✓ Selected: {formInitialPDF.name} ({(formInitialPDF.size / (1024 * 1024)).toFixed(2)} MB)
                      </p>
                    )}
                  </div>
                )}

                {/* Submit Buttons */}
                <div className="flex items-center justify-end gap-2 pt-4 border-t border-slate-100">
                  <button
                    type="button"
                    onClick={() => setCreateModalOpen(false)}
                    className="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50 transition"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={formSubmitting}
                    className="rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 hover:from-amber-400 hover:to-amber-500 px-5 py-2 text-xs font-black text-slate-950 shadow-md transition disabled:opacity-50"
                  >
                    {formSubmitting
                      ? 'Saving Judgment...'
                      : editingJudgment
                      ? 'Update Judgment'
                      : 'Create Official Judgment Draft'}
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}

        {/* Modal: Upload / Replace PDF */}
        {uploadModalJudgment && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 backdrop-blur-sm p-4 animate-in fade-in">
            <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl border border-slate-200">
              <div className="flex items-center justify-between pb-3 border-b border-slate-100 mb-4">
                <div className="flex items-center gap-2">
                  <Upload className="h-5 w-5 text-amber-500" />
                  <h3 className="text-sm font-extrabold text-slate-900">
                    Attach Official Judgment PDF
                  </h3>
                </div>
                <button
                  type="button"
                  onClick={() => setUploadModalJudgment(null)}
                  className="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-800"
                >
                  <X className="h-5 w-5" />
                </button>
              </div>

              <p className="text-xs font-bold text-slate-800 mb-1">
                {uploadModalJudgment.case_title}
              </p>
              <p className="text-[11px] font-mono text-slate-500 mb-4">
                {uploadModalJudgment.case_number}
              </p>

              {uploadError && (
                <div className="mb-4 flex items-center gap-2 rounded-xl bg-red-50 border border-red-200 p-3 text-xs font-bold text-red-700">
                  <AlertCircle className="h-4 w-4 shrink-0 text-red-600" />
                  <span>{uploadError}</span>
                </div>
              )}

              <div
                onClick={() => !uploading && uploadFileInputRef.current?.click()}
                className="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-slate-300 p-8 text-center cursor-pointer hover:border-amber-400 hover:bg-amber-50/20 transition"
              >
                {selectedFile ? (
                  <div>
                    <FileCheck className="h-10 w-10 text-emerald-600 mx-auto mb-2" />
                    <p className="text-xs font-extrabold text-slate-900 truncate max-w-xs">
                      {selectedFile.name}
                    </p>
                    <p className="text-[11px] text-slate-500 mt-0.5">
                      {(selectedFile.size / (1024 * 1024)).toFixed(2)} MB • PDF Document
                    </p>
                  </div>
                ) : (
                  <div>
                    <Upload className="h-10 w-10 text-amber-500 mx-auto mb-2" />
                    <p className="text-xs font-bold text-slate-800">
                      Click to browse official PDF
                    </p>
                    <p className="text-[11px] text-slate-400 mt-1">
                      Max 100MB • Magic bytes %PDF- verified • Virus scanned
                    </p>
                  </div>
                )}
              </div>

              <input
                ref={uploadFileInputRef}
                type="file"
                accept="application/pdf"
                className="hidden"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) setSelectedFile(file);
                }}
              />

              <div className="flex items-center justify-end gap-2 pt-4 border-t border-slate-100 mt-4">
                <button
                  type="button"
                  onClick={() => setUploadModalJudgment(null)}
                  disabled={uploading}
                  className="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleUploadSubmit}
                  disabled={!selectedFile || uploading}
                  className="rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 hover:from-amber-400 hover:to-amber-500 px-5 py-2 text-xs font-black text-slate-950 shadow-md disabled:opacity-50 transition flex items-center gap-1.5"
                >
                  {uploading ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Uploading &amp; Scanning...
                    </>
                  ) : (
                    'Upload & Attach'
                  )}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Modal: Publish & Allocate Citation */}
        {publishModalJudgment && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 backdrop-blur-sm p-4 animate-in fade-in">
            <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl border border-slate-200">
              <div className="flex items-center justify-between pb-3 border-b border-slate-100 mb-4">
                <div className="flex items-center gap-2">
                  <Award className="h-5 w-5 text-amber-500" />
                  <h3 className="text-sm font-black text-slate-900">
                    Publish Official Citation
                  </h3>
                </div>
                <button
                  type="button"
                  onClick={() => setPublishModalJudgment(null)}
                  className="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-800"
                >
                  <X className="h-5 w-5" />
                </button>
              </div>

              <div className="rounded-xl bg-amber-50/60 border border-amber-200 p-4 mb-4 text-xs">
                <p className="font-extrabold text-amber-950 mb-1">
                  Atomic Sequential Citation Allocation
                </p>
                <p className="text-amber-800 leading-relaxed text-[11px]">
                  Publishing this judgment will lock the database counter for{' '}
                  <span className="font-mono font-bold">{publishModalJudgment.court_code}</span> and{' '}
                  allocate the next gapless number in sequence:
                </p>
                <div className="mt-2 text-center py-2 px-3 bg-white rounded-lg border border-amber-300 font-mono font-bold text-amber-950 text-xs">
                  {new Date().getFullYear()} {brandPrefix} ({publishModalJudgment.court_code}) [NEXT_ID]
                </div>
              </div>

              <div className="space-y-3 mb-5">
                <div>
                  <label className="block text-xs font-bold text-slate-700 mb-1">
                    Brand Citation Prefix
                  </label>
                  <input
                    type="text"
                    value={brandPrefix}
                    onChange={(e) => setBrandPrefix(e.target.value)}
                    placeholder="CourtDesk"
                    className="w-full rounded-xl border border-slate-200 bg-slate-50/50 p-2 text-xs font-bold text-slate-900 focus:border-amber-400 focus:outline-none"
                  />
                  <p className="text-[10px] text-slate-400 mt-1">
                    Unique branding prefix for official report citations.
                  </p>
                </div>

                <div>
                  <label className="block text-xs font-bold text-slate-700 mb-1">
                    Judgment to Publish
                  </label>
                  <p className="text-xs font-extrabold text-slate-900">
                    {publishModalJudgment.case_title}
                  </p>
                  <p className="text-[11px] font-mono text-slate-500">
                    {publishModalJudgment.case_number}
                  </p>
                </div>
              </div>

              <div className="flex items-center justify-end gap-2 pt-4 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => setPublishModalJudgment(null)}
                  disabled={formSubmitting}
                  className="rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handlePublishClick}
                  disabled={formSubmitting}
                  className="rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 hover:from-amber-400 hover:to-amber-500 px-5 py-2 text-xs font-black text-slate-950 shadow-md disabled:opacity-50 transition flex items-center gap-1.5"
                >
                  {formSubmitting ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Allocating...
                    </>
                  ) : (
                    'Confirm & Publish'
                  )}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Modal: View Extracted Pages */}
        {pagesModalJudgment && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 backdrop-blur-sm p-4 animate-in fade-in">
            <div className="w-full max-w-4xl rounded-2xl bg-white p-6 shadow-2xl border border-slate-200 my-8 max-h-[90vh] flex flex-col">
              <div className="flex items-center justify-between pb-3 border-b border-slate-100 mb-4 shrink-0">
                <div className="flex items-center gap-2">
                  <Layers className="h-5 w-5 text-amber-500" />
                  <div>
                    <h3 className="text-sm font-extrabold text-slate-900">
                      Extracted Judgment Pages
                    </h3>
                    <p className="text-xs text-slate-500">
                      {pagesModalJudgment.case_title} • {pagesList.length} indexed pages
                    </p>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => setPagesModalJudgment(null)}
                  className="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-800"
                >
                  <X className="h-5 w-5" />
                </button>
              </div>

              {pagesLoading ? (
                <div className="py-20 text-center text-slate-400">
                  <Loader2 className="h-8 w-8 animate-spin mx-auto mb-2 text-amber-500" />
                  <p className="text-xs font-bold">Loading extracted page text...</p>
                </div>
              ) : pagesList.length === 0 ? (
                <div className="py-16 text-center text-slate-400">
                  <FileQuestion className="h-10 w-10 mx-auto mb-2 text-slate-300" />
                  <p className="text-xs font-bold text-slate-600">No extracted pages found</p>
                  <p className="text-[11px] text-slate-400 mt-1">
                    Text extraction worker is either pending or processing this PDF.
                  </p>
                </div>
              ) : (
                <div className="flex-1 overflow-y-auto space-y-4 pr-1">
                  {pagesList.map((p) => (
                    <div
                      key={p.id}
                      className="rounded-xl border border-slate-200 bg-slate-50/50 p-4 shadow-xs"
                    >
                      <div className="flex items-center justify-between pb-2 mb-2 border-b border-slate-200/80">
                        <span className="font-mono text-xs font-extrabold text-slate-800">
                          Page {p.page_number}
                        </span>
                        {p.has_ocr ? (
                          <span className="inline-flex items-center gap-1 rounded bg-blue-100 text-blue-900 px-2 py-0.5 text-[10px] font-bold">
                            <Sparkles className="h-3 w-3" />
                            OCR Scanned
                          </span>
                        ) : (
                          <span className="inline-flex items-center rounded bg-emerald-100 text-emerald-900 px-2 py-0.5 text-[10px] font-semibold">
                            Native PDF Text
                          </span>
                        )}
                      </div>
                      <pre className="font-mono text-xs leading-relaxed text-slate-800 whitespace-pre-wrap max-h-48 overflow-y-auto bg-white p-3 rounded-lg border border-slate-100">
                        {p.text_content || '(Empty page content)'}
                      </pre>
                    </div>
                  ))}
                </div>
              )}

              <div className="pt-4 border-t border-slate-100 mt-4 flex items-center justify-end shrink-0">
                <button
                  type="button"
                  onClick={() => setPagesModalJudgment(null)}
                  className="rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-700 px-4 py-2 text-xs font-bold transition"
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        )}
      </CMSLayout>
    </ProtectedRoute>
  );
}
