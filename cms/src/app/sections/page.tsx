'use client';

import React, { useState, useEffect } from 'react';
import CMSLayout from '@/components/layout/CMSLayout';
import apiClient from '@/lib/api/client';
import { useAuth } from '@/lib/auth/context';
import {
  FolderTree,
  Plus,
  Search,
  CheckCircle2,
  XCircle,
  Edit2,
  Trash2,
  Loader2,
  Layers,
  ArrowUpDown,
  AlertCircle,
  X,
} from 'lucide-react';

interface Section {
  id: string;
  name: string;
  slug: string;
  description?: string | null;
  section_type: 'court' | 'topic' | 'edition';
  parent_id?: string | null;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export default function SectionsPage() {
  const { hasRole } = useAuth();
  const [sections, setSections] = useState<Section[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [filterType, setFilterType] = useState<string>('all');
  const [error, setError] = useState<string | null>(null);

  // Modal State
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingSection, setEditingSection] = useState<Section | null>(null);
  const [modalLoading, setModalLoading] = useState(false);
  const [modalError, setModalError] = useState<string | null>(null);

  // Form State
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [sectionType, setSectionType] = useState<'court' | 'topic' | 'edition'>('court');
  const [sortOrder, setSortOrder] = useState<number>(10);
  const [isActive, setIsActive] = useState<boolean>(true);

  // Delete State
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const fetchSections = async () => {
    try {
      setLoading(true);
      setError(null);
      const { data, error, response } = await apiClient.GET('/v1/cms/sections');
      if (response.ok && data) {
        setSections((data as any).sections || []);
      } else {
        setError((error as any)?.message || 'Failed to load taxonomy sections.');
      }
    } catch (err: any) {
      setError(err.message || 'Network error fetching sections.');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSections();
  }, []);

  const openCreateModal = () => {
    setEditingSection(null);
    setName('');
    setSlug('');
    setDescription('');
    setSectionType('court');
    setSortOrder(sections.length * 10 + 10);
    setIsActive(true);
    setModalError(null);
    setIsModalOpen(true);
  };

  const openEditModal = (sec: Section) => {
    setEditingSection(sec);
    setName(sec.name);
    setSlug(sec.slug);
    setDescription(sec.description || '');
    setSectionType(sec.section_type);
    setSortOrder(sec.sort_order);
    setIsActive(sec.is_active);
    setModalError(null);
    setIsModalOpen(true);
  };

  const handleNameChange = (val: string) => {
    setName(val);
    if (!editingSection) {
      setSlug(
        val
          .toLowerCase()
          .replace(/[^a-z0-9]+/g, '-')
          .replace(/^-+|-+$/g, '')
      );
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !slug.trim()) {
      setModalError('Name and slug are required.');
      return;
    }

    setModalLoading(true);
    setModalError(null);

    try {
      if (editingSection) {
        // Update section
        const { data, error, response } = await apiClient.PUT('/v1/cms/sections/{id}', {
          params: { path: { id: editingSection.id } },
          body: {
            name: name.trim(),
            slug: slug.trim().toLowerCase(),
            description: description.trim() || undefined,
            section_type: sectionType,
            is_active: isActive,
            sort_order: Number(sortOrder) || 0,
          },
        });
        if (!response.ok || !data) {
          throw new Error((error as any)?.message || 'Failed to update section.');
        }
      } else {
        // Create section
        const { data, error, response } = await apiClient.POST('/v1/cms/sections', {
          body: {
            name: name.trim(),
            slug: slug.trim().toLowerCase(),
            description: description.trim() || undefined,
            section_type: sectionType,
            is_active: isActive,
            sort_order: Number(sortOrder) || 0,
          },
        });
        if (!response.ok || !data) {
          throw new Error((error as any)?.message || 'Failed to create section.');
        }
      }

      setIsModalOpen(false);
      fetchSections();
    } catch (err: any) {
      setModalError(err.message || 'Error saving section.');
    } finally {
      setModalLoading(false);
    }
  };

  const handleDelete = async (secId: string, secName: string) => {
    if (!confirm(`Are you sure you want to delete section "${secName}"?`)) {
      return;
    }
    setDeletingId(secId);
    try {
      const { response, error } = await apiClient.DELETE('/v1/cms/sections/{id}', {
        params: { path: { id: secId } },
      });
      if (!response.ok) {
        alert((error as any)?.message || 'Failed to delete section.');
      } else {
        fetchSections();
      }
    } catch (err: any) {
      alert(err.message || 'Network error deleting section.');
    } finally {
      setDeletingId(null);
    }
  };

  const filteredSections = sections.filter((s) => {
    const matchesSearch =
      s.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      s.slug.toLowerCase().includes(searchQuery.toLowerCase()) ||
      (s.description && s.description.toLowerCase().includes(searchQuery.toLowerCase()));
    const matchesType = filterType === 'all' || s.section_type === filterType;
    return matchesSearch && matchesType;
  });

  return (
    <CMSLayout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
              <FolderTree className="h-6 w-6 text-amber-500" />
              Taxonomy &amp; Sections
            </h1>
            <p className="text-sm text-slate-400 mt-1">
              Manage court jurisdictions, topical sections (Tax, IBC, Arbitration), and edition categories.
            </p>
          </div>
          {hasRole('editor', 'admin') && (
            <button
              onClick={openCreateModal}
              className="inline-flex items-center justify-center gap-2 px-4 py-2 bg-amber-500 hover:bg-amber-400 text-slate-950 font-semibold text-sm rounded-xl transition shadow-md shadow-amber-500/10"
            >
              <Plus className="h-4 w-4" />
              Add Section
            </button>
          )}
        </div>

        {/* Filter bar */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 bg-slate-900 border border-slate-800 p-3 rounded-2xl">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400" />
            <input
              type="text"
              placeholder="Filter sections by name, slug or keywords..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-xl pl-9 pr-4 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-amber-500"
            />
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-slate-400 font-medium whitespace-nowrap">Type:</span>
            <select
              value={filterType}
              onChange={(e) => setFilterType(e.target.value)}
              className="bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-amber-500"
            >
              <option value="all">All Types</option>
              <option value="court">Courts &amp; Tribunals</option>
              <option value="topic">Topics (IBC, Tax, etc.)</option>
              <option value="edition">Editions (Hindi, etc.)</option>
            </select>
          </div>
        </div>

        {/* Error message */}
        {error && (
          <div className="p-4 bg-red-500/10 border border-red-500/20 text-red-400 rounded-xl flex items-center gap-3 text-sm">
            <AlertCircle className="h-5 w-5 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {/* Sections Table */}
        <div className="bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
          {loading ? (
            <div className="py-20 flex flex-col items-center justify-center text-slate-400 gap-3">
              <Loader2 className="h-8 w-8 animate-spin text-amber-500" />
              <p className="text-sm">Loading taxonomy sections...</p>
            </div>
          ) : filteredSections.length === 0 ? (
            <div className="py-16 text-center text-slate-400">
              <Layers className="h-10 w-10 mx-auto text-slate-600 mb-2" />
              <p className="text-base font-semibold text-white">No sections found</p>
              <p className="text-sm text-slate-500 mt-1">
                {searchQuery || filterType !== 'all'
                  ? 'Try adjusting your search criteria.'
                  : 'Get started by creating your first editorial section.'}
              </p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm text-slate-300">
                <thead className="bg-slate-950/60 text-xs uppercase tracking-wider text-slate-400 border-b border-slate-800">
                  <tr>
                    <th className="px-5 py-3 font-semibold">Order</th>
                    <th className="px-5 py-3 font-semibold">Section Name &amp; Slug</th>
                    <th className="px-5 py-3 font-semibold">Type</th>
                    <th className="px-5 py-3 font-semibold">Description</th>
                    <th className="px-5 py-3 font-semibold">Status</th>
                    <th className="px-5 py-3 font-semibold text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60">
                  {filteredSections.map((sec) => (
                    <tr key={sec.id} className="hover:bg-slate-850/50 transition">
                      <td className="px-5 py-4 font-mono text-xs text-slate-400 font-semibold">
                        {sec.sort_order}
                      </td>
                      <td className="px-5 py-4">
                        <div className="font-semibold text-white">{sec.name}</div>
                        <div className="font-mono text-xs text-amber-400/80 mt-0.5">/{sec.slug}</div>
                      </td>
                      <td className="px-5 py-4">
                        <span
                          className={`inline-block px-2.5 py-1 text-xs font-semibold rounded-lg capitalize border ${
                            sec.section_type === 'court'
                              ? 'bg-blue-500/10 text-blue-400 border-blue-500/20'
                              : sec.section_type === 'topic'
                              ? 'bg-purple-500/10 text-purple-400 border-purple-500/20'
                              : 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                          }`}
                        >
                          {sec.section_type}
                        </span>
                      </td>
                      <td className="px-5 py-4 max-w-xs text-slate-400 text-xs truncate">
                        {sec.description || '—'}
                      </td>
                      <td className="px-5 py-4">
                        {sec.is_active ? (
                          <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-emerald-400">
                            <CheckCircle2 className="h-4 w-4" /> Active
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-500">
                            <XCircle className="h-4 w-4" /> Hidden
                          </span>
                        )}
                      </td>
                      <td className="px-5 py-4 text-right space-x-2">
                        {hasRole('editor', 'admin') && (
                          <button
                            onClick={() => openEditModal(sec)}
                            className="p-1.5 text-slate-400 hover:text-white hover:bg-slate-800 rounded-lg transition"
                            title="Edit Section"
                          >
                            <Edit2 className="h-4 w-4" />
                          </button>
                        )}
                        {hasRole('admin') && (
                          <button
                            onClick={() => handleDelete(sec.id, sec.name)}
                            disabled={deletingId === sec.id}
                            className="p-1.5 text-slate-400 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition disabled:opacity-50"
                            title="Delete Section"
                          >
                            {deletingId === sec.id ? (
                              <Loader2 className="h-4 w-4 animate-spin text-red-400" />
                            ) : (
                              <Trash2 className="h-4 w-4" />
                            )}
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Modal */}
        {isModalOpen && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/70 backdrop-blur-sm">
            <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-150">
              <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-950/50">
                <h3 className="font-bold text-white text-base">
                  {editingSection ? 'Edit Section' : 'Create New Section'}
                </h3>
                <button
                  onClick={() => setIsModalOpen(false)}
                  className="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800"
                >
                  <X className="h-5 w-5" />
                </button>
              </div>

              <form onSubmit={handleSubmit} className="p-6 space-y-4">
                {modalError && (
                  <div className="p-3 bg-red-500/10 border border-red-500/20 text-red-400 rounded-xl text-xs">
                    {modalError}
                  </div>
                )}

                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                    Section Name *
                  </label>
                  <input
                    type="text"
                    required
                    value={name}
                    onChange={(e) => handleNameChange(e.target.value)}
                    placeholder="e.g. Insolvency & Bankruptcy (IBC)"
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-white focus:outline-none focus:border-amber-500"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                    URL Slug *
                  </label>
                  <div className="flex items-center">
                    <span className="bg-slate-800 border border-r-0 border-slate-700 text-slate-400 px-3 py-2 text-sm rounded-l-xl font-mono">
                      /
                    </span>
                    <input
                      type="text"
                      required
                      value={slug}
                      onChange={(e) => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]+/g, ''))}
                      placeholder="ibc"
                      className="w-full bg-slate-950 border border-slate-800 rounded-r-xl px-3.5 py-2 text-sm text-white font-mono focus:outline-none focus:border-amber-500"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                      Section Type
                    </label>
                    <select
                      value={sectionType}
                      onChange={(e) => setSectionType(e.target.value as any)}
                      className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-amber-500"
                    >
                      <option value="court">Court</option>
                      <option value="topic">Topic</option>
                      <option value="edition">Edition</option>
                    </select>
                  </div>

                  <div>
                    <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                      Sort Order
                    </label>
                    <input
                      type="number"
                      value={sortOrder}
                      onChange={(e) => setSortOrder(parseInt(e.target.value) || 0)}
                      className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-white focus:outline-none focus:border-amber-500 font-mono"
                    />
                  </div>
                </div>

                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                    Description
                  </label>
                  <textarea
                    rows={2}
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    placeholder="Brief description for readers and SEO metadata..."
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-white focus:outline-none focus:border-amber-500"
                  />
                </div>

                <div className="flex items-center gap-3 pt-2">
                  <input
                    type="checkbox"
                    id="is_active_toggle"
                    checked={isActive}
                    onChange={(e) => setIsActive(e.target.checked)}
                    className="h-4 w-4 rounded border-slate-700 bg-slate-950 text-amber-500 focus:ring-amber-500"
                  />
                  <label htmlFor="is_active_toggle" className="text-sm font-medium text-slate-300">
                    Active (visible in reader navigation and section feeds)
                  </label>
                </div>

                <div className="flex items-center justify-end gap-3 pt-4 border-t border-slate-800">
                  <button
                    type="button"
                    onClick={() => setIsModalOpen(false)}
                    className="px-4 py-2 text-slate-400 hover:text-white text-sm font-medium transition"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={modalLoading}
                    className="inline-flex items-center gap-2 px-5 py-2 bg-amber-500 hover:bg-amber-400 text-slate-950 font-semibold text-sm rounded-xl transition shadow-md shadow-amber-500/10 disabled:opacity-50"
                  >
                    {modalLoading && <Loader2 className="h-4 w-4 animate-spin" />}
                    {editingSection ? 'Save Changes' : 'Create Section'}
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}
      </div>
    </CMSLayout>
  );
}
