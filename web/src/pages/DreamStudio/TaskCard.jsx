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
import { Card, Tag, Button, Typography, Space, Image, Modal } from '@douyinfe/semi-ui';
import { IconDelete, IconRefresh, IconEyeOpened } from '@douyinfe/semi-icons';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

const { Text, Paragraph } = Typography;

const TaskCard = ({ task, onDelete, onRegenerate, onViewDetail }) => {
  const { t } = useTranslation();
  const [imagePreviewVisible, setImagePreviewVisible] = useState(false);

  const getStatusTag = (status) => {
    const statusMap = {
      pending: { color: 'grey', text: t('等待中') },
      running: { color: 'blue', text: t('生成中') },
      succeeded: { color: 'green', text: t('成功') },
      failed: { color: 'red', text: t('失败') }
    };
    const config = statusMap[status] || statusMap.pending;
    return <Tag color={config.color}>{config.text}</Tag>;
  };

  const formatParams = (params) => {
    if (!params) return '';
    const parts = [];
    if (params.model) parts.push(params.model);
    if (params.size) parts.push(params.size);
    if (params.quality) parts.push(params.quality);
    if (params.style) parts.push(params.style);
    return parts.join(' · ');
  };

  return (
    <>
      <Card
        className="task-card"
        bodyStyle={{ padding: '16px' }}
        style={{ height: '100%', display: 'flex', flexDirection: 'column' }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '12px' }}>
          {getStatusTag(task.status)}
          <Text type="tertiary" size="small">
            {new Date(task.created_at).toLocaleString()}
          </Text>
        </div>

        {task.status === 'succeeded' && task.image_url && (
          <div
            style={{
              width: '100%',
              aspectRatio: '1',
              marginBottom: '12px',
              cursor: 'pointer',
              overflow: 'hidden',
              borderRadius: '8px'
            }}
            onClick={() => setImagePreviewVisible(true)}
          >
            <Image
              src={task.image_url}
              alt={task.prompt}
              width="100%"
              height="100%"
              style={{ objectFit: 'cover' }}
              preview={false}
            />
          </div>
        )}

        {task.status === 'failed' && task.error_message && (
          <div style={{ marginBottom: '12px', padding: '8px', backgroundColor: 'var(--semi-color-danger-light-default)', borderRadius: '4px' }}>
            <Text type="danger" size="small">{task.error_message}</Text>
          </div>
        )}

        <Paragraph
          ellipsis={{ rows: 2, showTooltip: true }}
          style={{ marginBottom: '8px', flex: 1 }}
        >
          {task.prompt}
        </Paragraph>

        <Text type="tertiary" size="small" style={{ marginBottom: '12px' }}>
          {formatParams(task.params)}
        </Text>

        <Space spacing="tight" style={{ width: '100%' }}>
          <Button
            icon={<IconEyeOpened />}
            size="small"
            onClick={() => onViewDetail?.(task)}
            style={{ flex: 1 }}
          >
            {t('详情')}
          </Button>
          {task.status === 'failed' && (
            <Button
              icon={<IconRefresh />}
              size="small"
              type="secondary"
              onClick={() => onRegenerate?.(task)}
              style={{ flex: 1 }}
            >
              {t('重试')}
            </Button>
          )}
          <Button
            icon={<IconDelete />}
            size="small"
            type="danger"
            onClick={() => onDelete?.(task.id)}
          />
        </Space>
      </Card>

      <Modal
        visible={imagePreviewVisible}
        onCancel={() => setImagePreviewVisible(false)}
        footer={null}
        width="auto"
        style={{ maxWidth: '90vw' }}
      >
        <Image
          src={task.image_url}
          alt={task.prompt}
          style={{ maxWidth: '100%', maxHeight: '80vh' }}
          preview={false}
        />
      </Modal>
    </>
  );
};

TaskCard.propTypes = {
  task: PropTypes.shape({
    id: PropTypes.string.isRequired,
    status: PropTypes.oneOf(['pending', 'running', 'succeeded', 'failed']).isRequired,
    prompt: PropTypes.string.isRequired,
    image_url: PropTypes.string,
    error_message: PropTypes.string,
    created_at: PropTypes.string.isRequired,
    params: PropTypes.object
  }).isRequired,
  onDelete: PropTypes.func,
  onRegenerate: PropTypes.func,
  onViewDetail: PropTypes.func
};

export default TaskCard;
