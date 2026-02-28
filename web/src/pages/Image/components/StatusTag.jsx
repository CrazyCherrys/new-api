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
import { Tag } from '@douyinfe/semi-ui';
import {
  IconClock,
  IconPlay,
  IconTickCircle,
  IconAlertCircle,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const StatusTag = ({ status, progress }) => {
  const { t } = useTranslation();

  const statusConfig = {
    pending: {
      color: 'orange',
      icon: <IconClock />,
      text: t('等待中'),
    },
    running: {
      color: 'blue',
      icon: <IconPlay />,
      text: progress ? `${t('生成中')} ${progress}%` : t('生成中'),
    },
    succeeded: {
      color: 'green',
      icon: <IconTickCircle />,
      text: t('已完成'),
    },
    failed: {
      color: 'red',
      icon: <IconAlertCircle />,
      text: t('失败'),
    },
  };

  const config = statusConfig[status] || statusConfig.pending;

  return (
    <Tag color={config.color} prefixIcon={config.icon} size="large">
      {config.text}
    </Tag>
  );
};

export default StatusTag;
