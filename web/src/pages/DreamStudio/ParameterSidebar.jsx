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

import React from 'react';
import { Form, Select, Slider, InputNumber, Tooltip } from '@douyinfe/semi-ui';
import { IconHelpCircle } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const ParameterSidebar = ({ state, dispatch }) => {
  const { t } = useTranslation();

  const modelOptions = [
    { value: 'stable-diffusion-xl-1024-v1-0', label: 'SDXL 1.0' },
    { value: 'stable-diffusion-v1-6', label: 'SD 1.6' },
    { value: 'stable-diffusion-512-v2-1', label: 'SD 2.1' },
  ];

  const resolutionOptions = [
    { value: '1024x1024', label: '1024×1024', width: 1024, height: 1024 },
    { value: '1152x896', label: '1152×896', width: 1152, height: 896 },
    { value: '896x1152', label: '896×1152', width: 896, height: 1152 },
    { value: '1216x832', label: '1216×832', width: 1216, height: 832 },
    { value: '832x1216', label: '832×1216', width: 832, height: 1216 },
    { value: '1344x768', label: '1344×768', width: 1344, height: 768 },
    { value: '768x1344', label: '768×1344', width: 768, height: 1344 },
    { value: '1536x640', label: '1536×640', width: 1536, height: 640 },
    { value: '640x1536', label: '640×1536', width: 640, height: 1536 },
  ];

  const aspectRatioOptions = [
    { value: '1:1', label: '1:1 (方形)', width: 1024, height: 1024 },
    { value: '16:9', label: '16:9 (横向)', width: 1344, height: 768 },
    { value: '9:16', label: '9:16 (纵向)', width: 768, height: 1344 },
    { value: '4:3', label: '4:3 (横向)', width: 1152, height: 896 },
    { value: '3:4', label: '3:4 (纵向)', width: 896, height: 1152 },
    { value: '21:9', label: '21:9 (超宽)', width: 1536, height: 640 },
  ];

  const handleResolutionChange = (value) => {
    const selected = resolutionOptions.find((opt) => opt.value === value);
    if (selected) {
      dispatch({
        type: 'SET_RESOLUTION',
        payload: { width: selected.width, height: selected.height },
      });
    }
  };

  const handleAspectRatioChange = (value) => {
    const selected = aspectRatioOptions.find((opt) => opt.value === value);
    if (selected) {
      dispatch({
        type: 'SET_RESOLUTION',
        payload: { width: selected.width, height: selected.height },
      });
    }
  };

  const currentResolution = `${state.width}x${state.height}`;

  return (
    <div className='space-y-6'>
      <Form.Select
        label={
          <span className='flex items-center gap-1'>
            {t('模型')}
            <Tooltip content={t('选择用于生成图像的 AI 模型')}>
              <IconHelpCircle size='small' className='text-gray-400' />
            </Tooltip>
          </span>
        }
        value={state.model}
        onChange={(value) => dispatch({ type: 'SET_MODEL', payload: value })}
        optionList={modelOptions}
        style={{ width: '100%' }}
      />

      <div>
        <Form.Label>
          <span className='flex items-center gap-1'>
            {t('采样步数')}
            <Tooltip content={t('生成图像的迭代次数，步数越多细节越丰富，但耗时更长')}>
              <IconHelpCircle size='small' className='text-gray-400' />
            </Tooltip>
          </span>
        </Form.Label>
        <Slider
          value={state.steps}
          onChange={(value) => dispatch({ type: 'SET_STEPS', payload: value })}
          min={1}
          max={50}
          step={1}
          showBoundary
        />
        <div className='text-right text-sm text-gray-500 mt-1'>{state.steps}</div>
      </div>

      <div>
        <Form.Label>
          <span className='flex items-center gap-1'>
            {t('提示词相关性')}
            <Tooltip content={t('控制生成结果与提示词的匹配程度，值越高越严格遵循提示词')}>
              <IconHelpCircle size='small' className='text-gray-400' />
            </Tooltip>
          </span>
        </Form.Label>
        <Slider
          value={state.cfgScale}
          onChange={(value) => dispatch({ type: 'SET_CFG_SCALE', payload: value })}
          min={1}
          max={20}
          step={0.5}
          showBoundary
        />
        <div className='text-right text-sm text-gray-500 mt-1'>{state.cfgScale}</div>
      </div>

      <Form.InputNumber
        label={
          <span className='flex items-center gap-1'>
            {t('种子值')}
            <Tooltip content={t('控制随机性的数值，相同种子和参数会生成相同图像。-1 表示随机')}>
              <IconHelpCircle size='small' className='text-gray-400' />
            </Tooltip>
          </span>
        }
        value={state.seed}
        onChange={(value) => dispatch({ type: 'SET_SEED', payload: value })}
        min={-1}
        max={4294967295}
        style={{ width: '100%' }}
      />

      <Form.Select
        label={
          <span className='flex items-center gap-1'>
            {t('分辨率')}
            <Tooltip content={t('选择生成图像的像素尺寸')}>
              <IconHelpCircle size='small' className='text-gray-400' />
            </Tooltip>
          </span>
        }
        value={currentResolution}
        onChange={handleResolutionChange}
        optionList={resolutionOptions}
        style={{ width: '100%' }}
      />

      <Form.Select
        label={
          <span className='flex items-center gap-1'>
            {t('宽高比')}
            <Tooltip content={t('快速选择常用的图像宽高比例')}>
              <IconHelpCircle size='small' className='text-gray-400' />
            </Tooltip>
          </span>
        }
        value={
          aspectRatioOptions.find(
            (opt) => opt.width === state.width && opt.height === state.height
          )?.value || ''
        }
        onChange={handleAspectRatioChange}
        optionList={aspectRatioOptions}
        style={{ width: '100%' }}
        placeholder={t('自定义')}
      />

      <Form.InputNumber
        label={
          <span className='flex items-center gap-1'>
            {t('生成数量')}
            <Tooltip content={t('一次生成的图像数量，最多 4 张')}>
              <IconHelpCircle size='small' className='text-gray-400' />
            </Tooltip>
          </span>
        }
        value={state.samples}
        onChange={(value) => dispatch({ type: 'SET_SAMPLES', payload: value })}
        min={1}
        max={4}
        style={{ width: '100%' }}
      />
    </div>
  );
};

export default ParameterSidebar;
