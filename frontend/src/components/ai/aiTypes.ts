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

export enum ChatMode {
  ASK = 1,
  AGENT = 2,
}

export interface PageContext {
  page_type: string;
  run_id?: string;
  pipeline_id?: string;
  pipeline_version_id?: string;
  experiment_id?: string;
  namespace?: string;
}

export interface ChatRequest {
  message: string;
  session_id: string;
  mode: ChatMode;
  page_context?: PageContext;
}

export interface ChatResponseEvent {
  type: string;
  data: any;
}

export interface MarkdownChunkData {
  content: string;
}

export interface ToolCallData {
  tool_call_id: string;
  tool_name: string;
  arguments_json?: string;
  read_only: boolean;
}

export interface ToolResultData {
  tool_call_id: string;
  result_json: string;
  success: boolean;
  error?: string;
}

export interface ConfirmationRequestData {
  tool_call_id: string;
  tool_name: string;
  description: string;
  arguments_json: string;
}

export interface ProgressData {
  message: string;
  percentage: number;
}

export interface SessionMetadataData {
  session_id: string;
  model: string;
  available_tools: string[];
}

export interface ErrorData {
  message: string;
  code: string;
  retryable: boolean;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  toolCalls?: ToolCallInfo[];
  timestamp: Date;
}

export interface ToolCallInfo {
  id: string;
  name: string;
  arguments?: string;
  result?: string;
  success?: boolean;
  readOnly: boolean;
  status: 'pending' | 'executing' | 'completed' | 'denied';
}

export interface PendingConfirmation {
  toolCallId: string;
  toolName: string;
  description: string;
  argumentsJson: string;
}
