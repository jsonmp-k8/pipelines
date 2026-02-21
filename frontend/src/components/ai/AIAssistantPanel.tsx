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
import CloseIcon from '@material-ui/icons/Close';
import Resizable from 're-resizable';
import Slide from '@material-ui/core/Slide';
import { color, zIndex } from '../../Css';
import { stylesheet } from 'typestyle';
import { ChatMessages } from './ChatMessages';
import { ChatInput } from './ChatInput';
import { ModeSelector } from './ModeSelector';
import { ConfirmationDialog } from './ConfirmationDialog';
import { useAIChat } from './useAIChat';
import { ChatMode, PageContext } from './aiTypes';

const css = stylesheet({
  closeButton: {
    color: color.inactive,
    margin: 8,
    minHeight: 0,
    minWidth: 0,
    padding: 4,
  },
  header: {
    alignItems: 'center',
    borderBottom: `1px solid ${color.divider}`,
    display: 'flex',
    padding: '4px 8px',
  },
  panel: {
    backgroundColor: color.background,
    borderLeft: `1px solid ${color.divider}`,
    bottom: 0,
    display: 'flex',
    flexDirection: 'column',
    position: 'absolute !important' as any,
    right: 0,
    top: 0,
    zIndex: zIndex.SIDE_PANEL,
  },
  title: {
    flex: 1,
    fontSize: 14,
    fontWeight: 500,
    textAlign: 'center',
  },
  content: {
    display: 'flex',
    flex: 1,
    flexDirection: 'column',
    overflow: 'hidden',
  },
  errorBanner: {
    backgroundColor: color.errorBg,
    color: color.errorText,
    fontSize: 13,
    padding: '8px 16px',
  },
});

interface AIAssistantPanelProps {
  isOpen: boolean;
  onClose: () => void;
  initialPrompt?: string;
  pageContext?: PageContext;
}

const AIAssistantPanel: React.FC<AIAssistantPanelProps> = ({
  isOpen,
  onClose,
  initialPrompt,
  pageContext,
}) => {
  const {
    messages,
    isStreaming,
    mode,
    pendingConfirmation,
    error,
    sendMessage,
    approveToolCallAction,
    setMode,
    cancelStream,
  } = useAIChat();

  const initialPromptSentRef = React.useRef(false);

  React.useEffect(() => {
    if (isOpen && initialPrompt && !initialPromptSentRef.current) {
      initialPromptSentRef.current = true;
      sendMessage(initialPrompt, pageContext);
    }
  }, [isOpen, initialPrompt, pageContext, sendMessage]);

  React.useEffect(() => {
    if (!isOpen) {
      initialPromptSentRef.current = false;
    }
  }, [isOpen]);

  // Cancel the stream when the panel is closed
  React.useEffect(() => {
    if (!isOpen) {
      cancelStream();
    }
  }, [isOpen, cancelStream]);

  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && isOpen) {
        onClose();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  const handleSendMessage = React.useCallback(
    (text: string) => {
      sendMessage(text, pageContext);
    },
    [sendMessage, pageContext],
  );

  return (
    <Slide in={isOpen} direction='left'>
      <Resizable
        className={css.panel}
        defaultSize={{ width: 420 }}
        maxWidth='60%'
        minWidth={320}
        enable={{
          bottom: false,
          bottomLeft: false,
          bottomRight: false,
          left: true,
          right: false,
          top: false,
          topLeft: false,
          topRight: false,
        }}
      >
        {isOpen && (
          <>
            <div className={css.header}>
              <Button aria-label='close' className={css.closeButton} onClick={onClose}>
                <CloseIcon />
              </Button>
              <div className={css.title}>AI Assistant</div>
              <ModeSelector mode={mode} onModeChange={setMode} disabled={isStreaming} />
            </div>

            {error && <div className={css.errorBanner}>{error}</div>}

            <div className={css.content}>
              <ChatMessages messages={messages} isStreaming={isStreaming} />

              {pendingConfirmation && (
                <ConfirmationDialog
                  toolName={pendingConfirmation.toolName}
                  description={pendingConfirmation.description}
                  argumentsJson={pendingConfirmation.argumentsJson}
                  onApprove={() => approveToolCallAction(pendingConfirmation.toolCallId, true)}
                  onDeny={() => approveToolCallAction(pendingConfirmation.toolCallId, false)}
                />
              )}

              <ChatInput
                onSend={handleSendMessage}
                disabled={isStreaming}
                placeholder={
                  mode === ChatMode.ASK
                    ? 'Ask about your pipelines...'
                    : 'Ask or instruct the agent...'
                }
              />
            </div>
          </>
        )}
      </Resizable>
    </Slide>
  );
};

export default AIAssistantPanel;
