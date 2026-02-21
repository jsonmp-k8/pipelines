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

import { useCallback, useRef, useState } from 'react';
import { streamChat, approveToolCall } from '../../lib/AiApi';
import {
  ChatMessage,
  ChatMode,
  ChatResponseEvent,
  ConfirmationRequestData,
  PageContext,
  PendingConfirmation,
  ToolCallInfo,
} from './aiTypes';

const messageCounterRef = { current: 0 };

function generateMessageId(): string {
  return `msg-${Date.now()}-${++messageCounterRef.current}`;
}

export interface UseAIChatReturn {
  messages: ChatMessage[];
  isStreaming: boolean;
  sessionId: string;
  mode: ChatMode;
  pendingConfirmation: PendingConfirmation | null;
  error: string | null;
  sendMessage: (text: string, pageContext?: PageContext) => Promise<void>;
  approveToolCallAction: (toolCallId: string, approved: boolean) => Promise<void>;
  setMode: (mode: ChatMode) => void;
  clearMessages: () => void;
  cancelStream: () => void;
}

export function useAIChat(): UseAIChatReturn {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [sessionId, setSessionId] = useState('');
  const [mode, setMode] = useState<ChatMode>(ChatMode.ASK);
  const [pendingConfirmation, setPendingConfirmation] = useState<PendingConfirmation | null>(null);
  const [error, setError] = useState<string | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  // Use refs to avoid stale closures in sendMessage
  const sessionIdRef = useRef(sessionId);
  sessionIdRef.current = sessionId;
  const modeRef = useRef(mode);
  modeRef.current = mode;
  const isStreamingRef = useRef(isStreaming);
  isStreamingRef.current = isStreaming;

  const cancelStream = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
  }, []);

  const processEvent = useCallback(
    (event: ChatResponseEvent, assistantMessageId: string) => {
      switch (event.type) {
        case 'session_metadata': {
          const data = event.data;
          if (data.session_id) {
            setSessionId(data.session_id);
          }
          break;
        }

        case 'markdown_chunk': {
          const content = event.data.content || '';
          setMessages(prev =>
            prev.map(msg =>
              msg.id === assistantMessageId
                ? { ...msg, content: msg.content + content }
                : msg,
            ),
          );
          break;
        }

        case 'tool_call': {
          const data = event.data;
          const toolCall: ToolCallInfo = {
            id: data.tool_call_id,
            name: data.tool_name,
            arguments: data.arguments_json,
            readOnly: data.read_only,
            status: 'executing',
          };
          setMessages(prev =>
            prev.map(msg => {
              if (msg.id !== assistantMessageId) return msg;
              const existing = msg.toolCalls?.find(tc => tc.id === data.tool_call_id);
              if (existing) {
                return {
                  ...msg,
                  toolCalls: msg.toolCalls?.map(tc =>
                    tc.id === data.tool_call_id ? { ...tc, ...toolCall } : tc,
                  ),
                };
              }
              return { ...msg, toolCalls: [...(msg.toolCalls || []), toolCall] };
            }),
          );
          break;
        }

        case 'tool_result': {
          const data = event.data;
          setMessages(prev =>
            prev.map(msg => {
              if (msg.id !== assistantMessageId) return msg;
              return {
                ...msg,
                toolCalls: msg.toolCalls?.map(tc =>
                  tc.id === data.tool_call_id
                    ? { ...tc, result: data.result_json, success: data.success, status: 'completed' as const }
                    : tc,
                ),
              };
            }),
          );
          break;
        }

        case 'confirmation_request': {
          const data = event.data as ConfirmationRequestData;
          setPendingConfirmation({
            toolCallId: data.tool_call_id,
            toolName: data.tool_name,
            description: data.description,
            argumentsJson: data.arguments_json,
          });
          break;
        }

        case 'error': {
          const data = event.data;
          setError(data.message);
          break;
        }

        case 'progress':
          // Progress events can be used for UI indicators
          break;
      }
    },
    [],
  );

  const sendMessage = useCallback(
    async (text: string, pageContext?: PageContext) => {
      if (isStreamingRef.current) return;

      setError(null);
      setIsStreaming(true);

      // Create and store abort controller
      const controller = new AbortController();
      abortControllerRef.current = controller;

      // Add user message
      const userMessage: ChatMessage = {
        id: generateMessageId(),
        role: 'user',
        content: text,
        timestamp: new Date(),
      };
      setMessages(prev => [...prev, userMessage]);

      // Create assistant message placeholder
      const assistantMessageId = generateMessageId();
      const assistantMessage: ChatMessage = {
        id: assistantMessageId,
        role: 'assistant',
        content: '',
        toolCalls: [],
        timestamp: new Date(),
      };
      setMessages(prev => [...prev, assistantMessage]);

      try {
        const stream = streamChat(
          {
            message: text,
            session_id: sessionIdRef.current,
            mode: modeRef.current,
            page_context: pageContext,
          },
          controller.signal,
        );

        for await (const event of stream) {
          processEvent(event, assistantMessageId);
        }
      } catch (err: unknown) {
        if (err instanceof DOMException && err.name === 'AbortError') {
          // User cancelled the stream, not an error
          return;
        }
        const message = err instanceof Error ? err.message : 'An error occurred';
        setError(message);
        setMessages(prev =>
          prev.map(msg =>
            msg.id === assistantMessageId
              ? { ...msg, content: msg.content || 'An error occurred while processing your request.' }
              : msg,
          ),
        );
      } finally {
        abortControllerRef.current = null;
        setIsStreaming(false);
      }
    },
    [processEvent],
  );

  const approveToolCallAction = useCallback(
    async (toolCallId: string, approved: boolean) => {
      try {
        await approveToolCall(sessionIdRef.current, toolCallId, approved);
        setPendingConfirmation(null);
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : 'Failed to process approval';
        setError(message);
      }
    },
    [],
  );

  const clearMessages = useCallback(() => {
    cancelStream();
    setMessages([]);
    setSessionId('');
    setError(null);
    setPendingConfirmation(null);
  }, [cancelStream]);

  return {
    messages,
    isStreaming,
    sessionId,
    mode,
    pendingConfirmation,
    error,
    sendMessage,
    approveToolCallAction,
    setMode,
    clearMessages,
    cancelStream,
  };
}
