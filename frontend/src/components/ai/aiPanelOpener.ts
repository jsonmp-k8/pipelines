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

import { PageContext } from './aiTypes';

type PanelOpener = (prompt?: string, pageContext?: PageContext) => void;

let registeredOpener: PanelOpener | null = null;

/**
 * Register the AI panel opener callback. Called by the Router's SideNavLayout
 * when the context provider mounts. This enables non-React code (e.g., Buttons.ts)
 * to open the AI panel without relying on window globals.
 */
export function registerAIPanelOpener(opener: PanelOpener): void {
  registeredOpener = opener;
}

/**
 * Unregister the AI panel opener callback. Called when the provider unmounts.
 */
export function unregisterAIPanelOpener(): void {
  registeredOpener = null;
}

/**
 * Open the AI panel with an optional prompt and page context.
 * Safe to call even if the AI feature is disabled (will be a no-op).
 */
export function openAIPanel(prompt?: string, pageContext?: PageContext): void {
  if (registeredOpener) {
    registeredOpener(prompt, pageContext);
  }
}
