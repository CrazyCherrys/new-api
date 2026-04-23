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
import { Table, Tag, Typography, Popconfirm, Button, Space } from '@douyinfe/semi-ui';
import { IconEdit, IconDelete } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { renderTimestamp } from '../../../helpers/render';

const { Text } = Typography;

const ModelMappingTable = ({
  mappings,
  loading,
  openEditModal,
  deleteMapping,
}) => {
  const { t } = useTranslation();

  const getModelTypeTag = (type) => {
    const typeMap = {
      1: { text: t('对话'), color: 'blue' },
      2: { text: t('绘画'), color: 'purple' },
      3: { text: t('视频'), color: 'orange' },
      4: { text: t('音频'), color: 'green' },
    };
    const config = typeMap[type] || { text: t('未知'), color: 'grey' };
    return <Tag color={config.color}>{config.text}</Tag>;
  };

  const getStatusTag = (status) => {
    return status === 1 ? (
      <Tag color='green'>{t('启用')}</Tag>
    ) : (
      <Tag color='red'>{t('禁用')}</Tag>
    );
  };

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 80,
    },
    {
      title: t('请求模型ID'),
      dataIndex: 'request_model',
      render: (text) => (
        <Text copyable onClick={(e) => e.stopPropagation()}>
          {text}
        </Text>
      ),
    },
    {
      title: t('实际调用模型ID'),
      dataIndex: 'actual_model',
      render: (text) => (
        <Text copyable onClick={(e) => e.stopPropagation()}>
          {text}
        </Text>
      ),
    },
    {
      title: t('显示名称'),
      dataIndex: 'display_name',
    },
    {
      title: t('模型系列'),
      dataIndex: 'model_series',
      render: (text) => text || '-',
    },
    {
      title: t('模型类型'),
      dataIndex: 'model_type',
      render: (type) => getModelTypeTag(type),
    },
    {
      title: t('请求端点'),
      dataIndex: 'request_endpoint',
      render: (text) => text || '-',
    },
    {
      title: t('分辨率'),
      dataIndex: 'resolutions',
      render: (text) => {
        if (!text) return '-';
        try {
          const resolutions = JSON.parse(text);
          return resolutions.join(', ');
        } catch (e) {
          return '-';
        }
      },
    },
    {
      title: t('宽高比'),
      dataIndex: 'aspect_ratios',
      render: (text) => {
        if (!text) return '-';
        try {
          const ratios = JSON.parse(text);
          return ratios.join(', ');
        } catch (e) {
          return '-';
        }
      },
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (status) => getStatusTag(status),
    },
    {
      title: t('优先级'),
      dataIndex: 'priority',
      width: 100,
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_time',
      render: (timestamp) => renderTimestamp(timestamp),
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      width: 150,
      render: (text, record) => (
        <Space>
          <Button
            theme='light'
            type='primary'
            size='small'
            icon={<IconEdit />}
            onClick={() => openEditModal(record)}
          >
            {t('编辑')}
          </Button>
          <Popconfirm
            title={t('确定删除此映射吗？')}
            onConfirm={() => deleteMapping(record.id)}
            okType='danger'
          >
            <Button
              theme='light'
              type='danger'
              size='small'
              icon={<IconDelete />}
            >
              {t('删除')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Table
      columns={columns}
      dataSource={mappings}
      loading={loading}
      pagination={false}
      rowKey='id'
    />
  );
};

export default ModelMappingTable;
