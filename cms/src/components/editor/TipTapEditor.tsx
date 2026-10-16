'use client';

import React, { useEffect } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import LinkExtension from '@tiptap/extension-link';
import Placeholder from '@tiptap/extension-placeholder';
import {
  Bold,
  Italic,
  Strikethrough,
  Heading2,
  Heading3,
  List,
  ListOrdered,
  Quote,
  Link as LinkIcon,
  Undo2,
  Redo2,
  Minus,
} from 'lucide-react';

interface TipTapEditorProps {
  content?: any;
  onChange?: (blocks: any[], html: string) => void;
  placeholder?: string;
  readOnly?: boolean;
}

export default function TipTapEditor({
  content,
  onChange,
  placeholder = 'Write courtroom reporting, legal analysis, judgment excerpts...',
  readOnly = false,
}: TipTapEditorProps) {
  const editor = useEditor({
    immediatelyRender: false,
    extensions: [
      StarterKit.configure({
        heading: {
          levels: [2, 3],
        },
      }),
      LinkExtension.configure({
        openOnClick: false,
        HTMLAttributes: {
          class: 'text-amber-600 underline font-semibold hover:text-amber-700',
        },
      }),
      Placeholder.configure({
        placeholder,
      }),
    ],
    content:
      Array.isArray(content) && content.length === 0
        ? ''
        : Array.isArray(content)
        ? { type: 'doc', content }
        : content || '',
    editable: !readOnly,
    onUpdate: ({ editor }) => {
      if (onChange) {
        const json = editor.getJSON();
        const blocks = json.content || [];
        const html = editor.getHTML();
        onChange(blocks, html);
      }
    },
  });

  // Sync external content update if editor content is empty or changed
  useEffect(() => {
    if (editor && content && editor.isEmpty) {
      if (Array.isArray(content)) {
        editor.commands.setContent({ type: 'doc', content });
      } else {
        editor.commands.setContent(content);
      }
    }
  }, [content, editor]);

  if (!editor) {
    return (
      <div className="min-h-[280px] rounded-xl border border-slate-200 bg-white p-4 animate-pulse flex items-center justify-center text-slate-400 text-sm">
        Initializing editorial rich text editor...
      </div>
    );
  }

  const setLink = () => {
    const previousUrl = editor.getAttributes('link').href;
    const url = window.prompt('Enter legal reference URL', previousUrl);
    if (url === null) return;
    if (url === '') {
      editor.chain().focus().extendMarkRange('link').unsetLink().run();
      return;
    }
    editor.chain().focus().extendMarkRange('link').setLink({ href: url }).run();
  };

  return (
    <div className="rounded-xl border border-slate-200 bg-white overflow-hidden shadow-sm focus-within:border-amber-400 focus-within:ring-1 focus-within:ring-amber-400 transition-all">
      {/* Editor Toolbar */}
      {!readOnly && (
        <div className="flex flex-wrap items-center gap-1 border-b border-slate-200 bg-slate-50/80 px-3 py-2 text-slate-700">
          <button
            type="button"
            onClick={() => editor.chain().focus().toggleBold().run()}
            disabled={!editor.can().chain().focus().toggleBold().run()}
            className={`p-1.5 rounded-lg text-xs font-semibold transition ${
              editor.isActive('bold')
                ? 'bg-amber-100 text-amber-900'
                : 'hover:bg-slate-200/80 text-slate-600'
            }`}
            title="Bold (Ctrl+B)"
          >
            <Bold className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={() => editor.chain().focus().toggleItalic().run()}
            disabled={!editor.can().chain().focus().toggleItalic().run()}
            className={`p-1.5 rounded-lg text-xs font-semibold transition ${
              editor.isActive('italic')
                ? 'bg-amber-100 text-amber-900'
                : 'hover:bg-slate-200/80 text-slate-600'
            }`}
            title="Italic (Ctrl+I)"
          >
            <Italic className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={() => editor.chain().focus().toggleStrike().run()}
            disabled={!editor.can().chain().focus().toggleStrike().run()}
            className={`p-1.5 rounded-lg text-xs font-semibold transition ${
              editor.isActive('strike')
                ? 'bg-amber-100 text-amber-900'
                : 'hover:bg-slate-200/80 text-slate-600'
            }`}
            title="Strikethrough"
          >
            <Strikethrough className="h-4 w-4" />
          </button>

          <div className="h-4 w-[1px] bg-slate-300 mx-1" />

          <button
            type="button"
            onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
            className={`p-1.5 rounded-lg text-xs font-semibold transition ${
              editor.isActive('heading', { level: 2 })
                ? 'bg-amber-100 text-amber-900'
                : 'hover:bg-slate-200/80 text-slate-600'
            }`}
            title="Heading 2"
          >
            <Heading2 className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}
            className={`p-1.5 rounded-lg text-xs font-semibold transition ${
              editor.isActive('heading', { level: 3 })
                ? 'bg-amber-100 text-amber-900'
                : 'hover:bg-slate-200/80 text-slate-600'
            }`}
            title="Heading 3"
          >
            <Heading3 className="h-4 w-4" />
          </button>

          <div className="h-4 w-[1px] bg-slate-300 mx-1" />

          <button
            type="button"
            onClick={() => editor.chain().focus().toggleBulletList().run()}
            className={`p-1.5 rounded-lg text-xs font-semibold transition ${
              editor.isActive('bulletList')
                ? 'bg-amber-100 text-amber-900'
                : 'hover:bg-slate-200/80 text-slate-600'
            }`}
            title="Bullet List"
          >
            <List className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={() => editor.chain().focus().toggleOrderedList().run()}
            className={`p-1.5 rounded-lg text-xs font-semibold transition ${
              editor.isActive('orderedList')
                ? 'bg-amber-100 text-amber-900'
                : 'hover:bg-slate-200/80 text-slate-600'
            }`}
            title="Numbered List"
          >
            <ListOrdered className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={() => editor.chain().focus().toggleBlockquote().run()}
            className={`p-1.5 rounded-lg text-xs font-semibold transition ${
              editor.isActive('blockquote')
                ? 'bg-amber-100 text-amber-900'
                : 'hover:bg-slate-200/80 text-slate-600'
            }`}
            title="Courtroom / Judgment Quote"
          >
            <Quote className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={setLink}
            className={`p-1.5 rounded-lg text-xs font-semibold transition ${
              editor.isActive('link')
                ? 'bg-amber-100 text-amber-900'
                : 'hover:bg-slate-200/80 text-slate-600'
            }`}
            title="Insert Link"
          >
            <LinkIcon className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={() => editor.chain().focus().setHorizontalRule().run()}
            className="p-1.5 rounded-lg text-xs font-semibold hover:bg-slate-200/80 text-slate-600 transition"
            title="Horizontal Divider"
          >
            <Minus className="h-4 w-4" />
          </button>

          <div className="h-4 w-[1px] bg-slate-300 mx-1" />

          <button
            type="button"
            onClick={() => editor.chain().focus().undo().run()}
            disabled={!editor.can().chain().focus().undo().run()}
            className="p-1.5 rounded-lg text-xs font-semibold hover:bg-slate-200/80 text-slate-600 transition disabled:opacity-40"
            title="Undo"
          >
            <Undo2 className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={() => editor.chain().focus().redo().run()}
            disabled={!editor.can().chain().focus().redo().run()}
            className="p-1.5 rounded-lg text-xs font-semibold hover:bg-slate-200/80 text-slate-600 transition disabled:opacity-40"
            title="Redo"
          >
            <Redo2 className="h-4 w-4" />
          </button>
        </div>
      )}

      {/* Editor Content Surface */}
      <div className="p-4 sm:p-5 min-h-[320px] max-h-[600px] overflow-y-auto prose max-w-none focus:outline-none">
        <EditorContent editor={editor} className="outline-none" />
      </div>

      {/* Footer Word & Character Counter */}
      <div className="flex items-center justify-between border-t border-slate-100 bg-slate-50/50 px-4 py-2 text-[11px] font-medium text-slate-400">
        <span>Rich Text Editorial Editor</span>
        <div className="flex items-center gap-3">
          <span>
            {editor.storage.characterCount ? editor.storage.characterCount.words() : editor.getText().split(/\s+/).filter(Boolean).length} words
          </span>
          <span>{editor.getText().length} chars</span>
        </div>
      </div>
    </div>
  );
}
