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
import { ChatMessage as ChatMessageType } from './aiTypes';
import { ToolCallCard } from './ToolCallCard';
import { MarkdownRenderer } from './MarkdownRenderer';

const css = stylesheet({
  container: {
    marginBottom: 12,
  },
  userMessage: {
    backgroundColor: color.activeBg,
    borderRadius: '12px 12px 4px 12px',
    fontSize: 13,
    lineHeight: '1.5',
    marginLeft: 40,
    padding: '8px 12px',
    wordBreak: 'break-word',
  },
  assistantMessage: {
    fontSize: 13,
    lineHeight: '1.5',
    padding: '4px 0',
    wordBreak: 'break-word',
  },
  roleLabel: {
    color: color.lowContrast,
    fontSize: 11,
    fontWeight: 500,
    marginBottom: 4,
    textTransform: 'uppercase',
  },
});

interface ChatMessageProps {
  message: ChatMessageType;
}

export const ChatMessage: React.FC<ChatMessageProps> = ({ message }) => {
  if (message.role === 'user') {
    return (
      <div className={css.container}>
        <div className={css.userMessage}>{message.content}</div>
      </div>
    );
  }

  return (
    <div className={css.container}>
      {message.content && (
        <div className={css.assistantMessage}>
          <MarkdownRenderer content={message.content} />
        </div>
      )}
      {message.toolCalls?.map(tc => (
        <ToolCallCard key={tc.id} toolCall={tc} />
      ))}
    </div>
  );
};
