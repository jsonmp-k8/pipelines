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
import { ToolCallInfo } from './aiTypes';

const css = stylesheet({
  card: {
    backgroundColor: color.whiteSmoke,
    border: `1px solid ${color.divider}`,
    borderRadius: 8,
    fontSize: 12,
    margin: '4px 0',
    overflow: 'hidden',
  },
  header: {
    alignItems: 'center',
    cursor: 'pointer',
    display: 'flex',
    padding: '6px 10px',
    $nest: {
      '&:hover': {
        backgroundColor: color.lightGrey,
      },
    },
  },
  toolName: {
    color: color.theme,
    flex: 1,
    fontFamily: 'monospace',
    fontWeight: 500,
  },
  statusBadge: {
    borderRadius: 4,
    fontSize: 10,
    fontWeight: 500,
    padding: '2px 6px',
    textTransform: 'uppercase',
  },
  executing: {
    backgroundColor: '#fff3e0',
    color: '#e65100',
  },
  completed: {
    backgroundColor: color.successWeak,
    color: '#1b5e20',
  },
  denied: {
    backgroundColor: color.errorBg,
    color: color.errorText,
  },
  pending: {
    backgroundColor: color.infoBg,
    color: color.infoText,
  },
  details: {
    borderTop: `1px solid ${color.divider}`,
    padding: '8px 10px',
  },
  detailLabel: {
    color: color.lowContrast,
    fontSize: 11,
    fontWeight: 500,
    marginBottom: 2,
  },
  codeBlock: {
    backgroundColor: '#f5f5f5',
    border: `1px solid ${color.divider}`,
    borderRadius: 4,
    fontFamily: 'monospace',
    fontSize: 11,
    lineHeight: '1.4',
    maxHeight: 150,
    overflow: 'auto',
    padding: 8,
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
  },
  arrow: {
    color: color.lowContrast,
    fontSize: 14,
    marginRight: 4,
    transition: 'transform 0.2s',
  },
  arrowExpanded: {
    transform: 'rotate(90deg)',
  },
});

interface ToolCallCardProps {
  toolCall: ToolCallInfo;
}

export const ToolCallCard: React.FC<ToolCallCardProps> = ({ toolCall }) => {
  const [expanded, setExpanded] = React.useState(false);

  const statusClass = {
    executing: css.executing,
    completed: css.completed,
    denied: css.denied,
    pending: css.pending,
  }[toolCall.status];

  const formatJSON = (jsonStr?: string): string => {
    if (!jsonStr) return '';
    try {
      return JSON.stringify(JSON.parse(jsonStr), null, 2);
    } catch {
      return jsonStr;
    }
  };

  return (
    <div className={css.card}>
      <div className={css.header} onClick={() => setExpanded(!expanded)}>
        <span className={`${css.arrow} ${expanded ? css.arrowExpanded : ''}`}>&#9654;</span>
        <span className={css.toolName}>{toolCall.name}</span>
        <span className={`${css.statusBadge} ${statusClass}`}>{toolCall.status}</span>
      </div>
      {expanded && (
        <div className={css.details}>
          {toolCall.arguments && (
            <>
              <div className={css.detailLabel}>Arguments</div>
              <pre className={css.codeBlock}>{formatJSON(toolCall.arguments)}</pre>
            </>
          )}
          {toolCall.result && (
            <>
              <div className={css.detailLabel} style={{ marginTop: 8 }}>
                Result {toolCall.success === false && '(Error)'}
              </div>
              <pre className={css.codeBlock}>{formatJSON(toolCall.result)}</pre>
            </>
          )}
        </div>
      )}
    </div>
  );
};
