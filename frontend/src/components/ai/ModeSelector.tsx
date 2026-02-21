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
import ButtonGroup from '@material-ui/core/ButtonGroup';
import { stylesheet } from 'typestyle';
import { color } from '../../Css';
import { ChatMode } from './aiTypes';

const css = stylesheet({
  container: {
    display: 'flex',
  },
  activeButton: {
    backgroundColor: color.theme,
    color: '#fff',
    fontSize: 11,
    padding: '2px 10px',
    textTransform: 'none',
    $nest: {
      '&:hover': {
        backgroundColor: color.themeDarker,
      },
    },
  },
  inactiveButton: {
    backgroundColor: 'transparent',
    color: color.grey,
    fontSize: 11,
    padding: '2px 10px',
    textTransform: 'none',
  },
});

interface ModeSelectorProps {
  mode: ChatMode;
  onModeChange: (mode: ChatMode) => void;
  disabled?: boolean;
}

export const ModeSelector: React.FC<ModeSelectorProps> = ({ mode, onModeChange, disabled }) => {
  return (
    <div className={css.container}>
      <ButtonGroup size='small' disabled={disabled}>
        <Button
          className={mode === ChatMode.ASK ? css.activeButton : css.inactiveButton}
          onClick={() => onModeChange(ChatMode.ASK)}
        >
          Ask
        </Button>
        <Button
          className={mode === ChatMode.AGENT ? css.activeButton : css.inactiveButton}
          onClick={() => onModeChange(ChatMode.AGENT)}
        >
          Agent
        </Button>
      </ButtonGroup>
    </div>
  );
};
