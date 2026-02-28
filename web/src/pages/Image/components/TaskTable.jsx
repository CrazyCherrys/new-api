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
import { Table, Empty, Typography, Tooltip } from '@douyinfe/semi-ui';
import { IconImage } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import StatusTag from './StatusTag';
import ImageActionGroup from './ImageActionGroup';

const { Text } = Typography;

const TaskTable = ({ tasks, loading, onRegenerate, onDelete }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  const columns = [
    {
      title: t('任务ID'),
      dataIndex: 'id',
      width: 100,
      render: (text) => (
        <Text type="tertiary" size="small">
          #{text}
        </Text>
      ),
    },
    {
      title: t('模型'),
      dataIndex: 'model',
      width: 150,
      render: (text) => <Text strong>{text}</Text>,
    },
    {
      title: t('提示词'),
      dataIndex: 'prompt',
      ellipsis: { showTitle: false },
      render: (text) => (
        <Tooltip content={text} position="top">
          <Text
            ellipsis={{ rows: 2 }}
            style={{ maxWidth: '300px', display: 'block' }}
          >
            {text}
          </Text>
        </Tooltip>
      ),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 140,
      render: (status, record) => (
        <StatusTag status={status} progress={record.progress} />
      ),
    },
    {
      title: t('预览'),
      dataIndex: 'result_url',
      width: 100,
      render: (url, record) => {
        if (record.status === 'succeeded' && url) {
          return (
            <img
              src={url}
              alt="preview"
              style={{
                width: '60px',
                height: '60px',
                objectFit: 'cover',
                borderRadius: '4px',
                cursor: 'pointer',
              }}
            />
          );
        }
        return (
          <div
            style={{
              width: '60px',
              height: '60px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              backgroundColor: '#f5f5f5',
              borderRadius: '4px',
            }}
          >
            <IconImage style={{ color: '#ccc' }} />
          </div>
        );
      },
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_at',
      width: 180,
      render: (timestamp) => {
        if (!timestamp) return '-';
        const date = new Date(timestamp * 1000);
        return (
          <Text type="tertiary" size="small">
            {date.toLocaleString('zh-CN', {
              year: 'numeric',
              month: '2-digit',
              day: '2-digit',
              hour: '2-digit',
              minute: '2-digit',
            })}
          </Text>
        );
      },
    },
    {
      title: t('操作'),
      dataIndex: 'actions',
      width: 180,
      fixed: 'right',
      render: (_, record) => (
        <ImageActionGroup
          task={record}
          onRegenerate={onRegenerate}
          onDelete={onDelete}
        />
      ),
    },
  ];

  // 移动端简化列
  const mobileColumns = [
    {
      title: t('任务信息'),
      dataIndex: 'info',
      render: (_, record) => (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between' }}>
            <Text type="tertiary" size="small">
              #{record.id}
            </Text>
            <StatusTag status={record.status} progress={record.progress} />
          </div>
          <Text strong>{record.model}</Text>
          <Text
            ellipsis={{ rows: 2 }}
            style={{ fontSize: '12px', color: '#666' }}
          >
            {record.prompt}
          </Text>
          {record.status === 'succeeded' && record.result_url && (
            <img
              src={record.result_url}
              alt="preview"
              style={{
                width: '100%',
                maxHeight: '200px',
                objectFit: 'contain',
                borderRadius: '4px',
                marginTop: '8px',
              }}
            />
          )}
          <div style={{ marginTop: '8px' }}>
            <ImageActionGroup
              task={record}
              onRegenerate={onRegenerate}
              onDelete={onDelete}
            />
          </div>
        </div>
      ),
    },
  ];

  if (!tasks || tasks.length === 0) {
    return (
      <Empty
        image={<IconImage style={{ fontSize: 48, color: '#ccc' }} />}
        title={t('暂无任务')}
        description={t('开始生成图片后，任务记录将显示在这里')}
      />
    );
  }

  return (
    <Table
      columns={isMobile ? mobileColumns : columns}
      dataSource={tasks}
      loading={loading}
      pagination={false}
      rowKey="id"
      size="small"
      style={{ marginTop: '16px' }}
    />
  );
};

export default TaskTable;
