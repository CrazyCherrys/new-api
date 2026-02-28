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
import { TextArea, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const PromptArea = ({ value, onChange, disabled, maxLength = 4000 }) => {
  const { t } = useTranslation();

  return (
    <div className="w-full">
      <div className="flex justify-between items-center mb-2">
        <Text strong>{t('提示词')}</Text>
        <Text type="tertiary" size="small">
          {value?.length || 0} / {maxLength}
        </Text>
      </div>
      <TextArea
        value={value}
        onChange={onChange}
        disabled={disabled}
        placeholder={t('描述你想要生成的图像，例如：一只可爱的橘猫坐在窗台上，阳光洒在它身上，油画风格')}
        maxLength={maxLength}
        autosize={{ minRows: 4, maxRows: 8 }}
        showClear
      />
    </div>
  );
};

export default PromptArea;
