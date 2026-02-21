/*
 * Copyright 2024 The Kubeflow Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import * as React from 'react';
import { stylesheet } from 'typestyle';
import { color } from '../../Css';

const css = stylesheet({
  markdown: {
    lineHeight: '1.6',
    $nest: {
      '& p': {
        margin: '4px 0',
      },
      '& code': {
        backgroundColor: color.whiteSmoke,
        borderRadius: 3,
        fontFamily: 'monospace',
        fontSize: '0.9em',
        padding: '1px 4px',
      },
      '& pre': {
        backgroundColor: '#1e1e1e',
        borderRadius: 6,
        color: '#d4d4d4',
        fontSize: 12,
        lineHeight: '1.4',
        margin: '8px 0',
        overflow: 'auto',
        padding: 12,
        $nest: {
          '& code': {
            backgroundColor: 'transparent',
            color: 'inherit',
            padding: 0,
          },
        },
      },
      '& ul, & ol': {
        margin: '4px 0',
        paddingLeft: 20,
      },
      '& li': {
        margin: '2px 0',
      },
      '& strong': {
        fontWeight: 600,
      },
      '& h1, & h2, & h3': {
        fontSize: 14,
        fontWeight: 600,
        margin: '12px 0 4px',
      },
      '& h1': {
        fontSize: 16,
      },
      '& a': {
        color: color.link,
      },
      '& blockquote': {
        borderLeft: `3px solid ${color.divider}`,
        color: color.lowContrast,
        margin: '8px 0',
        padding: '4px 12px',
      },
      '& table': {
        borderCollapse: 'collapse',
        margin: '8px 0',
        width: '100%',
      },
      '& th, & td': {
        border: `1px solid ${color.divider}`,
        fontSize: 12,
        padding: '4px 8px',
        textAlign: 'left',
      },
      '& th': {
        backgroundColor: color.whiteSmoke,
        fontWeight: 600,
      },
    },
  },
});

interface MarkdownRendererProps {
  content: string;
}

/**
 * Simple markdown renderer that converts basic markdown to HTML.
 * Uses a lightweight approach to avoid pulling in heavy dependencies.
 */
export const MarkdownRenderer: React.FC<MarkdownRendererProps> = ({ content }) => {
  const html = React.useMemo(() => renderMarkdown(content), [content]);

  return <div className={css.markdown} dangerouslySetInnerHTML={{ __html: html }} />;
};

function renderMarkdown(md: string): string {
  let html = escapeHtml(md);

  // Code blocks (``` ... ```)
  html = html.replace(/```(\w*)\n([\s\S]*?)```/g, (_match, lang, code) => {
    return `<pre><code class="language-${lang}">${code.trim()}</code></pre>`;
  });

  // Inline code
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>');

  // Bold
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');

  // Italic
  html = html.replace(/\*(.+?)\*/g, '<em>$1</em>');

  // Headers
  html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
  html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
  html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');

  // Unordered lists
  html = html.replace(/^[*-] (.+)$/gm, '<li>$1</li>');
  html = html.replace(/(<li>.*<\/li>\n?)+/g, match => `<ul>${match}</ul>`);

  // Ordered lists
  html = html.replace(/^\d+\. (.+)$/gm, '<li>$1</li>');

  // Blockquotes
  html = html.replace(/^&gt; (.+)$/gm, '<blockquote>$1</blockquote>');

  // Links — sanitize href to only allow safe protocols
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_match, text, href) => {
    const sanitizedHref = sanitizeHref(href);
    if (!sanitizedHref) {
      return text; // Render as plain text if href is unsafe
    }
    return `<a href="${sanitizedHref}" target="_blank" rel="noopener noreferrer">${text}</a>`;
  });

  // Paragraphs (double newlines)
  html = html.replace(/\n\n/g, '</p><p>');
  html = `<p>${html}</p>`;

  // Clean up empty paragraphs
  html = html.replace(/<p><\/p>/g, '');
  html = html.replace(/<p>(<h[123]>)/g, '$1');
  html = html.replace(/(<\/h[123]>)<\/p>/g, '$1');
  html = html.replace(/<p>(<pre>)/g, '$1');
  html = html.replace(/(<\/pre>)<\/p>/g, '$1');
  html = html.replace(/<p>(<ul>)/g, '$1');
  html = html.replace(/(<\/ul>)<\/p>/g, '$1');
  html = html.replace(/<p>(<blockquote>)/g, '$1');
  html = html.replace(/(<\/blockquote>)<\/p>/g, '$1');

  // Single newlines to <br> within paragraphs
  html = html.replace(/([^>])\n([^<])/g, '$1<br>$2');

  return html;
}

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function sanitizeHref(href: string): string | null {
  const trimmed = href.trim();
  // Only allow http, https, and relative URLs. Block javascript:, data:, vbscript:, etc.
  if (/^https?:\/\//i.test(trimmed) || /^[/#]/.test(trimmed)) {
    return trimmed;
  }
  // Block any scheme (e.g., javascript:, data:, vbscript:)
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(trimmed)) {
    return null;
  }
  // Allow relative URLs
  return trimmed;
}
