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
import { API } from '../../helpers/api';

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
  const [isOptimizing, setIsOptimizing] = useState(false);

  const optimizePrompt = async () => {
    if (!state.prompt.trim()) {
      Toast.warning(t('请输入提示词'));
      return;
    }

    setIsOptimizing(true);
    try {
      const response = await API.post('/api/v1/chat/completions', {
        model: 'gpt-4',
        messages: [
          {
            role: 'system',
            content: '你是一个图像生成 prompt 优化专家，帮助用户优化图像生成提示词，使其更加详细、准确、富有表现力。请直接返回优化后的 prompt，不要添加任何解释。'
          },
          { role: 'user', content: state.prompt }
        ]
      });

      if (response.data && response.data.choices && response.data.choices[0]) {
        const optimizedPrompt = response.data.choices[0].message.content;
        dispatch({ type: 'SET_PROMPT', payload: optimizedPrompt });
        Toast.success(t('提示词优化成功'));
      } else {
        Toast.error(t('优化失败，请重试'));
      }
    } catch (error) {
      Toast.error(error.message || t('优化失败，请重试'));
    } finally {
      setIsOptimizing(false);
    }
  };

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
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
          <label style={{ fontSize: '14px', fontWeight: 500 }}>{t('提示词')}</label>
          <span style={{ fontSize: '12px', color: '#8c8c8c' }}>
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
        <div style={{ marginTop: '8px' }}>
          <Button
            theme='light'
            type='tertiary'
            onClick={optimizePrompt}
            loading={isOptimizing}
            disabled={isOptimizing || !state.prompt.trim()}
          >
            {isOptimizing ? t('优化中...') : t('优化提示词')}
          </Button>
        </div>
      </div>

      <div>
        <div
          style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', cursor: 'pointer', marginBottom: '8px' }}
          onClick={() => setNegativeExpanded(!negativeExpanded)}
        >
          <label style={{ fontSize: '14px', fontWeight: 500 }}>{t('负向提示词')}</label>
          {negativeExpanded ? <IconChevronUp /> : <IconChevronDown />}
        </div>
        <Collapsible isOpen={negativeExpanded}>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: '8px' }}>
            <span style={{ fontSize: '12px', color: '#8c8c8c' }}>
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
        <label style={{ fontSize: '14px', fontWeight: 500, marginBottom: '8px', display: 'block' }}>{t('快速模板')}</label>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
          {QUICK_TEMPLATES.map((template, index) => (
            <Tag
              key={index}
              color='blue'
              onClick={() => applyTemplate(template)}
              style={{ cursor: 'pointer' }}
            >
              {t(template.label)}
            </Tag>
          ))}
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', paddingTop: '16px', borderTop: '1px solid #e8e8e8' }}>
        <div style={{ fontSize: '14px', color: '#666' }}>
          {t('预估成本')}: <span style={{ fontWeight: 600 }}>${estimateCost()}</span>
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
