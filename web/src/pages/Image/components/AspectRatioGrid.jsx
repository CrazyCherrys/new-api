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
import { Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const ASPECT_RATIOS = [
  { value: '1:1', label: '1:1', width: 1, height: 1 },
  { value: '16:9', label: '16:9', width: 16, height: 9 },
  { value: '9:16', label: '9:16', width: 9, height: 16 },
  { value: '4:3', label: '4:3', width: 4, height: 3 },
  { value: '3:4', label: '3:4', width: 3, height: 4 },
];

const AspectRatioGrid = ({ value, onChange, disabled }) => {
  const { t } = useTranslation();

  return (
    <div className="w-full">
      <Text strong className="block mb-2">
        {t('宽高比')}
      </Text>
      <div className="grid grid-cols-3 gap-2">
        {ASPECT_RATIOS.map((ratio) => (
          <button
            key={ratio.value}
            onClick={() => !disabled && onChange(ratio.value)}
            disabled={disabled}
            className={`
              relative p-4 rounded-lg border-2 transition-all
              ${
                value === ratio.value
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                  : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
              }
              ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
            `}
          >
            <div className="flex flex-col items-center justify-center">
              <div
                className={`
                  bg-gray-300 dark:bg-gray-600 rounded
                  ${value === ratio.value ? 'bg-blue-400 dark:bg-blue-500' : ''}
                `}
                style={{
                  width: `${Math.min(ratio.width * 8, 48)}px`,
                  height: `${Math.min(ratio.height * 8, 48)}px`,
                }}
              />
              <Text
                strong
                className={`mt-2 ${value === ratio.value ? 'text-blue-600 dark:text-blue-400' : ''}`}
              >
                {ratio.label}
              </Text>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
};

export default AspectRatioGrid;
