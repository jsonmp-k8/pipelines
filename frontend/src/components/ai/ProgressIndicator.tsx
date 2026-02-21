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
import { stylesheet, keyframes } from 'typestyle';
import { color } from '../../Css';

const bounce = keyframes({
  '0%, 80%, 100%': { transform: 'scale(0)' },
  '40%': { transform: 'scale(1)' },
});

const css = stylesheet({
  container: {
    alignItems: 'center',
    display: 'flex',
    gap: 4,
    padding: '8px 4px',
  },
  dot: {
    animationDuration: '1.4s',
    animationFillMode: 'both',
    animationIterationCount: 'infinite',
    animationName: bounce,
    backgroundColor: color.theme,
    borderRadius: '50%',
    display: 'inline-block',
    height: 6,
    width: 6,
  },
  dot1: {
    animationDelay: '-0.32s',
  },
  dot2: {
    animationDelay: '-0.16s',
  },
  dot3: {
    animationDelay: '0s',
  },
});

export const ProgressIndicator: React.FC = () => {
  return (
    <div className={css.container}>
      <span className={`${css.dot} ${css.dot1}`} />
      <span className={`${css.dot} ${css.dot2}`} />
      <span className={`${css.dot} ${css.dot3}`} />
    </div>
  );
};
