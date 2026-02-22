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
import Markdown from 'markdown-to-jsx';
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

const markdownOptions = {
  forceBlock: true,
  overrides: {
    a: {
      props: {
        target: '_blank',
        rel: 'noopener noreferrer',
      },
    },
  },
};

export const MarkdownRenderer: React.FC<MarkdownRendererProps> = ({ content }) => {
  return (
    <div className={css.markdown}>
      <Markdown options={markdownOptions}>{content}</Markdown>
    </div>
  );
};
