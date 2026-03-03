/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useReducer } from 'react';
import DreamStudioPanel from './DreamStudioPanel';

const initialState = {
  prompt: '',
  negativePrompt: '',
  model: 'stable-diffusion-xl-1024-v1-0',
  width: 1024,
  height: 1024,
  steps: 30,
  cfgScale: 7,
  seed: -1,
  samples: 1,
  sampler: 'K_DPMPP_2M',
  stylePreset: '',
  clipGuidancePreset: 'NONE',
  initImage: null,
  initImageMode: 'IMAGE_STRENGTH',
  imageStrength: 0.35,
  stepScheduleStart: 0.65,
  stepScheduleEnd: 1.0,
};

const reducer = (state, action) => {
  switch (action.type) {
    case 'SET_PROMPT':
      return { ...state, prompt: action.payload };
    case 'SET_NEGATIVE_PROMPT':
      return { ...state, negativePrompt: action.payload };
    case 'SET_MODEL':
      return { ...state, model: action.payload };
    case 'SET_RESOLUTION':
      return { ...state, width: action.payload.width, height: action.payload.height };
    case 'SET_WIDTH':
      return { ...state, width: action.payload };
    case 'SET_HEIGHT':
      return { ...state, height: action.payload };
    case 'SET_STEPS':
      return { ...state, steps: action.payload };
    case 'SET_CFG_SCALE':
      return { ...state, cfgScale: action.payload };
    case 'SET_SEED':
      return { ...state, seed: action.payload };
    case 'SET_SAMPLES':
      return { ...state, samples: action.payload };
    case 'SET_SAMPLER':
      return { ...state, sampler: action.payload };
    case 'SET_STYLE_PRESET':
      return { ...state, stylePreset: action.payload };
    case 'SET_CLIP_GUIDANCE_PRESET':
      return { ...state, clipGuidancePreset: action.payload };
    case 'SET_INIT_IMAGE':
      return { ...state, initImage: action.payload };
    case 'SET_INIT_IMAGE_MODE':
      return { ...state, initImageMode: action.payload };
    case 'SET_IMAGE_STRENGTH':
      return { ...state, imageStrength: action.payload };
    case 'SET_STEP_SCHEDULE_START':
      return { ...state, stepScheduleStart: action.payload };
    case 'SET_STEP_SCHEDULE_END':
      return { ...state, stepScheduleEnd: action.payload };
    case 'RESET':
      return initialState;
    default:
      return state;
  }
};

const DreamStudio = () => {
  const [state, dispatch] = useReducer(reducer, initialState);

  return (
    <div className='mt-[60px]'>
      <DreamStudioPanel state={state} dispatch={dispatch} />
    </div>
  );
};

export default DreamStudio;
