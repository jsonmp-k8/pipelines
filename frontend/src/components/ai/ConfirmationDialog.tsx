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
import Button from '@material-ui/core/Button';
import { stylesheet } from 'typestyle';
import { color } from '../../Css';

const css = stylesheet({
  container: {
    backgroundColor: color.warningBg,
    border: `1px solid ${color.alert}`,
    borderRadius: 8,
    margin: '8px 12px',
    padding: 12,
  },
  title: {
    fontSize: 13,
    fontWeight: 500,
    marginBottom: 8,
  },
  description: {
    color: color.secondaryText,
    fontSize: 12,
    marginBottom: 8,
  },
  codeBlock: {
    backgroundColor: '#f5f5f5',
    border: `1px solid ${color.divider}`,
    borderRadius: 4,
    fontFamily: 'monospace',
    fontSize: 11,
    lineHeight: '1.4',
    maxHeight: 120,
    overflow: 'auto',
    padding: 8,
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
  },
  actions: {
    display: 'flex',
    gap: 8,
    justifyContent: 'flex-end',
    marginTop: 12,
  },
  approveButton: {
    backgroundColor: color.success,
    color: '#fff',
    fontSize: 12,
    padding: '4px 16px',
    textTransform: 'none',
    $nest: {
      '&:hover': {
        backgroundColor: '#2e7d32',
      },
    },
  },
  denyButton: {
    borderColor: color.errorText,
    color: color.errorText,
    fontSize: 12,
    padding: '4px 16px',
    textTransform: 'none',
  },
});

interface ConfirmationDialogProps {
  toolName: string;
  description: string;
  argumentsJson: string;
  onApprove: () => void;
  onDeny: () => void;
}

export const ConfirmationDialog: React.FC<ConfirmationDialogProps> = ({
  toolName,
  description,
  argumentsJson,
  onApprove,
  onDeny,
}) => {
  const formatJSON = (jsonStr: string): string => {
    try {
      return JSON.stringify(JSON.parse(jsonStr), null, 2);
    } catch {
      return jsonStr;
    }
  };

  return (
    <div className={css.container}>
      <div className={css.title}>Confirm: {toolName}</div>
      <div className={css.description}>{description}</div>
      {argumentsJson && <pre className={css.codeBlock}>{formatJSON(argumentsJson)}</pre>}
      <div className={css.actions}>
        <Button className={css.denyButton} variant='outlined' onClick={onDeny}>
          Deny
        </Button>
        <Button className={css.approveButton} variant='contained' onClick={onApprove}>
          Approve
        </Button>
      </div>
    </div>
  );
};
