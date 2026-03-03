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

import React, { useState } from 'react';
import { TextArea, Button, Collapsible, Tag, Toast } from '@douyinfe/semi-ui';
import { IconChevronDown, IconChevronUp } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { createImageTask } from '../../helpers/imageApi';

const PROMPT_MAX_LENGTH = 2000;
const NEGATIVE_PROMPT_MAX_LENGTH = 2000;

const QUICK_TEMPLATES = [
  { label: '写实风格', prompt: 'photorealistic, highly detailed, 8k uhd, professional photography' },
  { label: '动漫风格', prompt: 'anime style, vibrant colors, detailed illustration, high quality' },
  { label: '油画风格', prompt: 'oil painting, artistic, classical art style, detailed brushwork' },
  { label: '赛博朋克', prompt: 'cyberpunk, neon lights, futuristic city, high tech, sci-fi' },
];

const PromptEditor = ({ state, dispatch, onTaskCreated }) => {
  const { t } = useTranslation();
  const [negativeExpanded, setNegativeExpanded] = useState(false);
  const [isGenerating, setIsGenerating] = useState(false);

  const handleGenerate = async () => {
    if (!state.prompt.trim()) {
      Toast.warning(t('请输入提示词'));
      return;
    }

    setIsGenerating(true);
    try {
      const payload = {
        prompt: state.prompt,
        negative_prompt: state.negativePrompt || undefined,
        model: state.model,
        width: state.width,
        height: state.height,
        steps: state.steps,
        cfg_scale: state.cfgScale,
        seed: state.seed === -1 ? undefined : state.seed,
        samples: state.samples,
        sampler: state.sampler,
        style_preset: state.stylePreset || undefined,
        clip_guidance_preset: state.clipGuidancePreset,
      };

      const response = await createImageTask(payload);

      if (response.success) {
        Toast.success(t('任务创建成功'));
        if (onTaskCreated) {
          onTaskCreated(response.data);
        }
      } else {
        Toast.error(response.message || t('任务创建失败'));
      }
    } catch (error) {
      Toast.error(error.message || t('网络错误，请重试'));
    } finally {
      setIsGenerating(false);
    }
  };

  const estimateCost = () => {
    const basePrice = 0.01;
    const pixelMultiplier = (state.width * state.height) / (1024 * 1024);
    const stepMultiplier = state.steps / 30;
    const sampleMultiplier = state.samples;
    return (basePrice * pixelMultiplier * stepMultiplier * sampleMultiplier).toFixed(4);
  };

  const applyTemplate = (template) => {
    dispatch({ type: 'SET_PROMPT', payload: template.prompt });
  };

  return (
    <div className='space-y-4'>
      <div>
        <div className='flex justify-between items-center mb-2'>
          <label className='text-sm font-medium'>{t('提示词')}</label>
          <span className='text-xs text-gray-500'>
            {state.prompt.length} / {PROMPT_MAX_LENGTH}
          </span>
        </div>
        <TextArea
          value={state.prompt}
          onChange={(value) => dispatch({ type: 'SET_PROMPT', payload: value })}
          placeholder={t('描述你想要生成的图片...')}
          autosize={{ minRows: 3, maxRows: 8 }}
          maxLength={PROMPT_MAX_LENGTH}
          showClear
        />
      </div>

      <div>
        <div
          className='flex items-center justify-between cursor-pointer mb-2'
          onClick={() => setNegativeExpanded(!negativeExpanded)}
        >
          <label className='text-sm font-medium'>{t('负向提示词')}</label>
          {negativeExpanded ? <IconChevronUp /> : <IconChevronDown />}
        </div>
        <Collapsible isOpen={negativeExpanded}>
          <div className='flex justify-end mb-2'>
            <span className='text-xs text-gray-500'>
              {state.negativePrompt.length} / {NEGATIVE_PROMPT_MAX_LENGTH}
            </span>
          </div>
          <TextArea
            value={state.negativePrompt}
            onChange={(value) => dispatch({ type: 'SET_NEGATIVE_PROMPT', payload: value })}
            placeholder={t('描述你不想在图片中出现的内容...')}
            autosize={{ minRows: 2, maxRows: 6 }}
            maxLength={NEGATIVE_PROMPT_MAX_LENGTH}
            showClear
          />
        </Collapsible>
      </div>

      <div>
        <label className='text-sm font-medium mb-2 block'>{t('快速模板')}</label>
        <div className='flex flex-wrap gap-2'>
          {QUICK_TEMPLATES.map((template, index) => (
            <Tag
              key={index}
              color='blue'
              onClick={() => applyTemplate(template)}
              className='cursor-pointer hover:opacity-80'
            >
              {t(template.label)}
            </Tag>
          ))}
        </div>
      </div>

      <div className='flex items-center justify-between pt-4 border-t'>
        <div className='text-sm text-gray-600'>
          {t('预估成本')}: <span className='font-semibold'>${estimateCost()}</span>
        </div>
        <Button
          theme='solid'
          type='primary'
          onClick={handleGenerate}
          loading={isGenerating}
          disabled={isGenerating || !state.prompt.trim()}
        >
          {isGenerating ? t('生成中...') : t('生成图片')}
        </Button>
      </div>
    </div>
  );
};

export default PromptEditor;
