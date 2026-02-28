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
import { RadioGroup, Radio, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const RESOLUTION_OPTIONS = [
  { value: '1024', label: '1K', description: '1024x1024' },
  { value: '2048', label: '2K', description: '2048x2048' },
  { value: '4096', label: '4K', description: '4096x4096' },
];

const ResolutionPicker = ({ value, onChange, disabled }) => {
  const { t } = useTranslation();

  return (
    <div className="w-full">
      <Text strong className="block mb-2">
        {t('分辨率')}
      </Text>
      <RadioGroup
        type="button"
        buttonSize="middle"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="w-full"
      >
        {RESOLUTION_OPTIONS.map((option) => (
          <Radio key={option.value} value={option.value} className="flex-1">
            <div className="flex flex-col items-center">
              <Text strong>{option.label}</Text>
              <Text type="tertiary" size="small">
                {option.description}
              </Text>
            </div>
          </Radio>
        ))}
      </RadioGroup>
    </div>
  );
};

export default ResolutionPicker;
