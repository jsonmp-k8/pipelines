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
import IconButton from '@material-ui/core/IconButton';
import SendIcon from '@material-ui/icons/Send';
import { stylesheet } from 'typestyle';
import { color } from '../../Css';

const css = stylesheet({
  container: {
    borderTop: `1px solid ${color.divider}`,
    display: 'flex',
    padding: '8px 12px',
  },
  textarea: {
    border: `1px solid ${color.divider}`,
    borderRadius: 8,
    flex: 1,
    fontFamily: 'inherit',
    fontSize: 13,
    lineHeight: '1.5',
    maxHeight: 120,
    outline: 'none',
    padding: '8px 12px',
    resize: 'none',
    $nest: {
      '&:focus': {
        borderColor: color.theme,
      },
    },
  },
  sendButton: {
    alignSelf: 'flex-end',
    color: color.theme,
    marginLeft: 4,
  },
  sendButtonDisabled: {
    color: color.disabledBg,
  },
});

interface ChatInputProps {
  onSend: (text: string) => void;
  disabled?: boolean;
  placeholder?: string;
}

export const ChatInput: React.FC<ChatInputProps> = ({
  onSend,
  disabled = false,
  placeholder = 'Type a message...',
}) => {
  const [text, setText] = React.useState('');
  const textareaRef = React.useRef<HTMLTextAreaElement>(null);

  const handleSend = React.useCallback(() => {
    const trimmed = text.trim();
    if (trimmed && !disabled) {
      onSend(trimmed);
      setText('');
      if (textareaRef.current) {
        textareaRef.current.style.height = 'auto';
      }
    }
  }, [text, disabled, onSend]);

  const handleKeyDown = React.useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend],
  );

  const handleInput = React.useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setText(e.target.value);
    // Auto-resize
    const el = e.target;
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, 120) + 'px';
  }, []);

  return (
    <div className={css.container}>
      <textarea
        ref={textareaRef}
        className={css.textarea}
        value={text}
        onChange={handleInput}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        disabled={disabled}
        rows={1}
      />
      <IconButton
        className={disabled || !text.trim() ? css.sendButtonDisabled : css.sendButton}
        onClick={handleSend}
        disabled={disabled || !text.trim()}
        size='small'
        aria-label='send'
      >
        <SendIcon fontSize='small' />
      </IconButton>
    </div>
  );
};
