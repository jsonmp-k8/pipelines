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
import Fab from '@material-ui/core/Fab';
import Tooltip from '@material-ui/core/Tooltip';
import { stylesheet } from 'typestyle';
import { color, zIndex } from '../../Css';
import { PageContext } from './aiTypes';
import { useAIAssistant } from './AIAssistantContext';

const css = stylesheet({
  fab: {
    backgroundColor: color.theme,
    bottom: 24,
    color: '#fff',
    position: 'fixed',
    right: 24,
    zIndex: zIndex.SIDE_PANEL - 1,
    $nest: {
      '&:hover': {
        backgroundColor: color.themeDarker,
      },
    },
  },
  icon: {
    fontSize: 20,
    fontWeight: 700,
  },
});

interface AIAssistantToggleProps {
  pageContext?: PageContext;
}

/**
 * A floating action button to open the AI assistant panel.
 */
export const AIAssistantToggle: React.FC<AIAssistantToggleProps> = ({ pageContext }) => {
  const { openPanel } = useAIAssistant();

  const handleClick = React.useCallback(() => {
    openPanel(undefined, pageContext);
  }, [openPanel, pageContext]);

  return (
    <Tooltip title='Open AI Assistant' placement='left'>
      <Fab className={css.fab} size='medium' onClick={handleClick} aria-label='AI Assistant'>
        <span className={css.icon}>AI</span>
      </Fab>
    </Tooltip>
  );
};
