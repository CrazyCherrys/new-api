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
import { Button, InputNumber, Typography, Space } from '@douyinfe/semi-ui';
import { IconImage } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const BatchSubmitter = ({ count, onCountChange, onSubmit, loading, disabled }) => {
  const { t } = useTranslation();

  return (
    <div className="w-full">
      <Space vertical align="start" spacing="medium" className="w-full">
        <div className="w-full">
          <Text strong className="block mb-2">
            {t('生成数量')}
          </Text>
          <InputNumber
            value={count}
            onChange={onCountChange}
            min={1}
            max={4}
            disabled={disabled || loading}
            style={{ width: '100%' }}
          />
          <Text type="tertiary" size="small" className="block mt-1">
            {t('一次最多生成 4 张图片')}
          </Text>
        </div>

        <Button
          theme="solid"
          type="primary"
          size="large"
          icon={<IconImage />}
          onClick={onSubmit}
          loading={loading}
          disabled={disabled}
          block
        >
          {loading ? t('生成中...') : t('生成图片')}
        </Button>
      </Space>
    </div>
  );
};

export default BatchSubmitter;
