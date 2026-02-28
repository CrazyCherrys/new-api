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
import { Select, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const MODEL_OPTIONS = [
  {
    value: 'dall-e-3',
    label: 'DALL-E 3',
    description: 'OpenAI 最新图像生成模型，高质量输出',
  },
  {
    value: 'dall-e-2',
    label: 'DALL-E 2',
    description: 'OpenAI 经典图像生成模型',
  },
  {
    value: 'stable-diffusion-xl',
    label: 'Stable Diffusion XL',
    description: '开源高质量图像生成模型',
  },
  {
    value: 'stable-diffusion-3',
    label: 'Stable Diffusion 3',
    description: 'Stability AI 最新模型',
  },
  {
    value: 'midjourney',
    label: 'Midjourney',
    description: '艺术风格图像生成',
  },
];

const ModelSelector = ({ value, onChange, disabled }) => {
  const { t } = useTranslation();

  const renderOptionItem = (option) => (
    <div className="flex flex-col py-1">
      <Text strong>{option.label}</Text>
      <Text type="tertiary" size="small">
        {t(option.description)}
      </Text>
    </div>
  );

  return (
    <div className="w-full">
      <Text strong className="block mb-2">
        {t('选择模型')}
      </Text>
      <Select
        value={value}
        onChange={onChange}
        disabled={disabled}
        placeholder={t('请选择图像生成模型')}
        style={{ width: '100%' }}
        optionList={MODEL_OPTIONS}
        renderSelectedItem={(option) => option.label}
        renderOptionItem={renderOptionItem}
      />
    </div>
  );
};

export default ModelSelector;
