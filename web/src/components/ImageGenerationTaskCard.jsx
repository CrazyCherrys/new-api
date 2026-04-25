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

import React, { useState, useEffect } from 'react';
import { Card, Progress, Spin, Typography, Tag, Checkbox } from '@douyinfe/semi-ui';
import { IconImage, IconAlertTriangle } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import PropTypes from 'prop-types';

const { Text } = Typography;

/**
 * 图片生成任务卡片组件
 *
 * 显示任务状态：
 * - pending/processing: 进度条 + loading动画 + 等待时长
 * - completed: 缩略图
 * - failed: 错误图标
 */
const ImageGenerationTaskCard = ({ task, onClick, selected, onSelectChange }) => {
  const { t } = useTranslation();
  const [waitTime, setWaitTime] = useState(0);

  // 计算等待时长（实时更新）
  useEffect(() => {
    if (task.status === 'pending' || task.status === 'generating') {
      const updateWaitTime = () => {
        const createdAt = new Date(task.created_time * 1000);
        const now = new Date();
        const diffSeconds = Math.floor((now - createdAt) / 1000);
        setWaitTime(diffSeconds);
      };

      updateWaitTime();
      const timer = setInterval(updateWaitTime, 1000);

      return () => clearInterval(timer);
    }
  }, [task.status, task.created_time]);

  // 格式化等待时长
  const formatWaitTime = (seconds) => {
    if (seconds < 60) {
      return `${seconds}${t('秒')}`;
    }
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;
    return `${minutes}${t('分')}${remainingSeconds}${t('秒')}`;
  };

  // 渲染卡片内容
  const renderContent = () => {
    switch (task.status) {
      case 'pending':
      case 'generating':
        return (
          <div className="flex flex-col items-center justify-center h-full p-4">
            <Spin size="large" />
            <Text className="mt-4" type="tertiary">
              {task.status === 'pending' ? t('等待中') : t('生成中...')}
            </Text>
            <Progress
              percent={task.progress || 0}
              showInfo
              className="w-full mt-2"
              stroke="var(--semi-color-primary)"
            />
            <Text className="mt-2" size="small" type="tertiary">
              {t('已等待')}: {formatWaitTime(waitTime)}
            </Text>
          </div>
        );

      case 'success':
        return (
          <div
            className="w-full h-full bg-cover bg-center bg-no-repeat cursor-pointer hover:opacity-80 transition-opacity"
            style={{
              backgroundImage: task.image_url ? `url(${task.image_url})` : 'none',
              backgroundColor: task.image_url ? 'transparent' : 'var(--semi-color-fill-0)',
            }}
          >
            {!task.image_url && (
              <div className="flex items-center justify-center h-full">
                <IconImage size="extra-large" style={{ color: 'var(--semi-color-text-2)' }} />
              </div>
            )}
          </div>
        );

      case 'failed':
        return (
          <div className="flex flex-col items-center justify-center h-full p-4 cursor-pointer hover:bg-gray-50 transition-colors">
            <IconAlertTriangle size="extra-large" style={{ color: 'var(--semi-color-danger)' }} />
            <Text className="mt-2" type="danger">
              {t('图片生成失败')}
            </Text>
            {task.error_message && (
              <Text className="mt-1 text-center" size="small" type="tertiary">
                {task.error_message}
              </Text>
            )}
          </div>
        );

      default:
        return null;
    }
  };

  // 获取状态标签
  const getStatusTag = () => {
    const statusMap = {
      pending: { color: 'blue', text: t('等待中') },
      generating: { color: 'cyan', text: t('生成中...') },
      success: { color: 'green', text: t('已完成') },
      failed: { color: 'red', text: t('失败') },
    };

    const status = statusMap[task.status] || { color: 'grey', text: task.status };
    return <Tag color={status.color} size="small">{status.text}</Tag>;
  };

  return (
    <Card
      className="!rounded-lg overflow-hidden"
      bodyStyle={{ padding: 0, height: '120px', position: 'relative' }}
      onClick={onClick}
      hoverable
      style={{ cursor: 'pointer' }}
    >
      {renderContent()}
      <div
        className="absolute top-2 left-2"
        style={{ zIndex: 10 }}
        onClick={(e) => e.stopPropagation()}
      >
        <Checkbox
          checked={selected}
          onChange={(e) => onSelectChange(task.id, e.target.checked)}
        />
      </div>
      <div className="absolute top-2 right-2" style={{ zIndex: 10 }}>
        {getStatusTag()}
      </div>
    </Card>
  );
};

ImageGenerationTaskCard.propTypes = {
  task: PropTypes.shape({
    id: PropTypes.number.isRequired,
    status: PropTypes.oneOf(['pending', 'generating', 'success', 'failed']).isRequired,
    image_url: PropTypes.string,
    progress: PropTypes.number,
    error_message: PropTypes.string,
    created_time: PropTypes.number.isRequired,
  }).isRequired,
  onClick: PropTypes.func,
  selected: PropTypes.bool,
  onSelectChange: PropTypes.func,
};

ImageGenerationTaskCard.defaultProps = {
  onClick: () => {},
  selected: false,
  onSelectChange: () => {},
};

export default ImageGenerationTaskCard;
