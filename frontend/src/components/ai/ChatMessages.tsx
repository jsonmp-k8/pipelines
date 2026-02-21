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
import { ChatMessage } from './ChatMessage';
import { ProgressIndicator } from './ProgressIndicator';

const css = stylesheet({
  container: {
    flex: 1,
    overflowY: 'auto',
    padding: '8px 12px',
  },
  emptyState: {
    alignItems: 'center',
    color: color.lowContrast,
    display: 'flex',
    flexDirection: 'column',
    fontSize: 14,
    justifyContent: 'center',
    padding: 40,
  },
});

interface ChatMessagesProps {
  messages: ChatMessageType[];
  isStreaming: boolean;
}

export const ChatMessages: React.FC<ChatMessagesProps> = ({ messages, isStreaming }) => {
  const containerRef = React.useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom on new messages
  React.useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [messages]);

  if (messages.length === 0) {
    return (
      <div className={css.container}>
        <div className={css.emptyState}>
          <div>Ask me about your pipelines, runs, or experiments.</div>
          <div style={{ marginTop: 8, fontSize: 12 }}>
            Switch to Agent mode to perform actions like creating runs.
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className={css.container} ref={containerRef}>
      {messages.map(msg => (
        <ChatMessage key={msg.id} message={msg} />
      ))}
      {isStreaming && <ProgressIndicator />}
    </div>
  );
};
