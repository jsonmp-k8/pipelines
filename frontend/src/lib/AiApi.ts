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

import { ChatRequest, ChatResponseEvent } from '../components/ai/aiTypes';

const AI_API_PREFIX = '/apis/v2beta1/ai';

/**
 * Stream a chat request via Server-Sent Events.
 * Returns an async generator that yields ChatResponseEvent objects.
 * Pass an AbortSignal to cancel the stream.
 */
export async function* streamChat(
  request: ChatRequest,
  signal?: AbortSignal,
): AsyncGenerator<ChatResponseEvent> {
  const response = await fetch(`${AI_API_PREFIX}/chat/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'same-origin',
    body: JSON.stringify(request),
    signal,
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`AI chat request failed: ${response.status} ${errorText}`);
  }

  const reader = response.body?.getReader();
  if (!reader) {
    throw new Error('Response body is not readable');
  }

  const decoder = new TextDecoder();
  let buffer = '';

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      // Process complete SSE events
      const lines = buffer.split('\n');
      buffer = lines.pop() || ''; // Keep incomplete line in buffer

      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed || !trimmed.startsWith('data: ')) continue;

        const data = trimmed.slice(6); // Remove 'data: ' prefix
        if (data === '[DONE]') return;

        try {
          const event: ChatResponseEvent = JSON.parse(data);
          yield event;
        } catch (e) {
          console.warn('Failed to parse SSE event:', data, e);
        }
      }
    }
  } finally {
    reader.releaseLock();
  }
}

/**
 * Approve or deny a pending tool call.
 */
export async function approveToolCall(
  sessionId: string,
  toolCallId: string,
  approved: boolean,
): Promise<{ success: boolean }> {
  const response = await fetch(`${AI_API_PREFIX}/approve`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'same-origin',
    body: JSON.stringify({
      session_id: sessionId,
      tool_call_id: toolCallId,
      approved,
    }),
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Approve tool call failed: ${response.status} ${errorText}`);
  }

  return response.json();
}

/**
 * Generate documentation for a pipeline.
 */
export async function generateDocumentation(
  pipelineId: string,
  pipelineVersionId?: string,
): Promise<{ documentation_markdown: string }> {
  const response = await fetch(`${AI_API_PREFIX}/generate-docs`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'same-origin',
    body: JSON.stringify({
      pipeline_id: pipelineId,
      pipeline_version_id: pipelineVersionId || '',
    }),
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Generate documentation failed: ${response.status} ${errorText}`);
  }

  return response.json();
}

/**
 * List AI rules.
 */
export async function listRules(): Promise<{ rules: any[] }> {
  const response = await fetch(`${AI_API_PREFIX}/rules`, {
    method: 'GET',
    credentials: 'same-origin',
  });

  if (!response.ok) {
    throw new Error(`List rules failed: ${response.status}`);
  }

  return response.json();
}

/**
 * Toggle an AI rule.
 */
export async function toggleRule(
  ruleId: string,
  enabled: boolean,
): Promise<{ rule: any }> {
  const response = await fetch(`${AI_API_PREFIX}/rules/${ruleId}:toggle`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'same-origin',
    body: JSON.stringify({
      rule_id: ruleId,
      enabled,
    }),
  });

  if (!response.ok) {
    throw new Error(`Toggle rule failed: ${response.status}`);
  }

  return response.json();
}
