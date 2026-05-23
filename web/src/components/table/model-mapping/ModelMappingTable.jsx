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
import {
  Table,
  Tag,
  Typography,
  Popconfirm,
  Button,
  Space,
  Switch,
} from '@douyinfe/semi-ui';
import { IconEdit, IconDelete } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';

const { Text } = Typography;
const DEFAULT_IMAGE_CAPABILITIES = ['image_generation', 'image_editing'];
const IMAGE_CAPABILITY_EDITING = 'image_editing';

const normalizeRequestEndpoint = (endpoint) => {
  const normalized = String(endpoint || '')
    .trim()
    .toLowerCase();
  switch (normalized) {
    case 'dalle':
      return 'openai';
    case 'claude':
      return 'anthropic';
    case 'openai-video-generations':
    case 'video-generation':
      return 'openai-video-generation';
    case 'openai-videos':
    case 'sora':
      return 'openai-video';
    default:
      return normalized;
  }
};

const renderTimestamp = (timestampInSeconds) => {
  const date = new Date(timestampInSeconds * 1000);
  const year = date.getFullYear();
  const month = ('0' + (date.getMonth() + 1)).slice(-2);
  const day = ('0' + date.getDate()).slice(-2);
  const hours = ('0' + date.getHours()).slice(-2);
  const minutes = ('0' + date.getMinutes()).slice(-2);
  const seconds = ('0' + date.getSeconds()).slice(-2);

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
};

const ModelMappingTable = ({
  mappings,
  loading,
  openEditModal,
  deleteMapping,
  refresh,
}) => {
  const { t } = useTranslation();

  const normalizeImageCapabilities = (raw) => {
    if (Array.isArray(raw)) {
      return raw;
    }
    if (typeof raw === 'string' && raw.trim() !== '') {
      try {
        const parsed = JSON.parse(raw);
        if (Array.isArray(parsed) && parsed.length > 0) {
          return parsed;
        }
      } catch (e) {
        return [...DEFAULT_IMAGE_CAPABILITIES];
      }
    }
    return [...DEFAULT_IMAGE_CAPABILITIES];
  };

  const getDefaultRequestEndpoint = (modelType) => {
    switch (Number(modelType)) {
      case 1:
        return 'openai';
      case 2:
        return 'openai';
      case 3:
        return 'openai-video-generation';
      default:
        return 'openai';
    }
  };

  const isValidRequestEndpointForModelType = (modelType, endpoint) => {
    const normalizedEndpoint = normalizeRequestEndpoint(endpoint);
    const endpointOptions = {
      1: ['openai', 'anthropic', 'gemini'],
      2: ['openai', 'openai-response', 'gemini'],
      3: ['openai-video-generation', 'openai-video'],
    };
    return (endpointOptions[Number(modelType)] || []).includes(
      normalizedEndpoint,
    );
  };

  const sanitizeMappingPayloadForUpdate = (record) => {
    const payload = { ...record };
    const modelType = Number(payload.model_type);
    payload.request_endpoint = normalizeRequestEndpoint(payload.request_endpoint);

    if (!isValidRequestEndpointForModelType(modelType, payload.request_endpoint)) {
      payload.request_endpoint = getDefaultRequestEndpoint(modelType);
    }
    if (modelType !== 2) {
      payload.image_capabilities = '';
      payload.reference_image_limit = 0;
    }
    if (modelType !== 3) {
      payload.video_capabilities = '';
      payload.duration_options = '';
    }
    if (!(modelType === 2 && payload.request_endpoint === 'gemini')) {
      payload.resolutions = '';
      payload.aspect_ratios = '';
    }
    if (modelType === 2) {
      const imageCapabilities = normalizeImageCapabilities(
        payload.image_capabilities,
      );
      payload.image_capabilities = JSON.stringify(imageCapabilities);
      if (!imageCapabilities.includes(IMAGE_CAPABILITY_EDITING)) {
        payload.reference_image_limit = 0;
      }
    }

    return payload;
  };

  const formatModelSeries = (series) => {
    if (!series) return '-';

    const seriesMap = {
      openai: 'OpenAI',
      gemini: 'Gemini',
      claude: 'Claude',
      grok: 'Grok',
      deepseek: 'DeepSeek',
      qwen: 'Qwen',
      glm: 'GLM',
      hunyuan: 'Hunyuan',
      doubao: 'Doubao',
      spark: 'Spark',
      baichuan: 'Baichuan',
      minimax: 'Minimax',
      moonshot: 'Moonshot',
      yi: 'Yi',
      chatglm: 'ChatGLM',
      ernie: 'ERNIE',
      wenxin: 'Wenxin',
      tongyi: 'Tongyi',
      azure: 'Azure',
      aws: 'AWS',
      cohere: 'Cohere',
      anthropic: 'Anthropic',
      mistral: 'Mistral',
      llama: 'Llama',
      palm: 'PaLM',
      bard: 'Bard',
      midjourney: 'Midjourney',
      'stable-diffusion': 'Stable Diffusion',
      flux: 'Flux',
      suno: 'Suno',
    };

    return (
      seriesMap[series.toLowerCase()] ||
      series.charAt(0).toUpperCase() + series.slice(1)
    );
  };

  const handleStatusToggle = async (record) => {
    try {
      const newStatus = record.status === 1 ? 0 : 1;
      const payload = sanitizeMappingPayloadForUpdate({
        ...record,
        status: newStatus,
      });
      const res = await API.put(`/api/model-mapping/`, {
        ...payload,
      });

      if (res.data.success) {
        showSuccess(t('状态更新成功'));
        refresh();
      } else {
        showError(res.data.message || t('状态更新失败'));
      }
    } catch (error) {
      showError(error.message || t('状态更新失败'));
    }
  };

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

  const formatRequestEndpoint = (endpoint, modelType) => {
    const normalizedEndpoint = normalizeRequestEndpoint(endpoint);
    if (modelType === 1) {
      const endpointMap = {
        openai: 'OpenAI (/v1/chat/completions)',
        anthropic: 'Claude (/v1/messages)',
        gemini: 'Gemini (/v1beta/models/{model}:generateContent)',
      };
      return endpointMap[normalizedEndpoint] || endpoint || '-';
    }

    if (modelType === 2) {
      const endpointMap = {
        openai: 'OpenAI (/v1/images)',
        dalle: 'OpenAI',
        gemini: 'Gemini 图片生成',
        'openai-response': 'OpenAI (/v1/responses)',
      };
      return endpointMap[normalizedEndpoint] || endpoint || '-';
    }

    const endpointMap = {
      openai: 'OpenAI',
      dalle: 'OpenAI',
      gemini: 'Gemini',
      'openai-video-generation':
        'OpenAI Video Generations (/v1/video/generations)',
      'openai-video': 'OpenAI Videos (Sora, /v1/videos)',
    };

    return endpointMap[normalizedEndpoint] || endpoint || '-';
  };

  const formatImageCapabilities = (raw) => {
    const capabilities = normalizeImageCapabilities(raw);
    const capabilityMap = {
      image_generation: t('图片生成'),
      image_editing: t('图像编辑'),
    };
    return capabilities
      .map((capability) => capabilityMap[capability] || capability)
      .join(', ');
  };

  const formatReferenceImageLimit = (record) => {
    if (record.model_type !== 2) {
      return '-';
    }
    const capabilities = normalizeImageCapabilities(record.image_capabilities);
    if (!capabilities.includes(IMAGE_CAPABILITY_EDITING)) {
      return '-';
    }
    const limit = Number(record.reference_image_limit) || 0;
    if (limit <= 0) {
      return t('参考图：不限制');
    }
    return t('参考图：最多 {{count}} 张', { count: limit });
  };

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 80,
    },
    {
      title: t('启用'),
      dataIndex: 'status',
      width: 80,
      render: (status, record) => (
        <Switch
          checked={status === 1}
          onChange={() => handleStatusToggle(record)}
        />
      ),
    },
    {
      title: t('模型ID'),
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
      render: (text) =>
        text ? (
          <Text copyable onClick={(e) => e.stopPropagation()}>
            {text}
          </Text>
        ) : (
          '-'
        ),
    },
    {
      title: t('显示名称'),
      dataIndex: 'display_name',
    },
    {
      title: t('模型系列'),
      dataIndex: 'model_series',
      render: (text) => formatModelSeries(text),
    },
    {
      title: t('模型类型'),
      dataIndex: 'model_type',
      render: (type) => getModelTypeTag(type),
    },
    {
      title: t('请求端点'),
      dataIndex: 'request_endpoint',
      render: (text, record) => formatRequestEndpoint(text, record.model_type),
    },
    {
      title: t('模型能力'),
      dataIndex: 'image_capabilities',
      render: (text, record) =>
        record.model_type === 2 ? (
          <Space vertical align='start' spacing={2}>
            <span>{formatImageCapabilities(text)}</span>
            <Text type='tertiary' size='small'>
              {formatReferenceImageLimit(record)}
            </Text>
          </Space>
        ) : (
          '-'
        ),
    },
    {
      title: t('分辨率'),
      dataIndex: 'resolutions',
      render: (text, record) => {
        if (
          record.model_type !== 2 ||
          normalizeRequestEndpoint(record.request_endpoint) !== 'gemini'
        ) {
          return '-';
        }
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
      render: (text, record) => {
        if (
          record.model_type !== 2 ||
          normalizeRequestEndpoint(record.request_endpoint) !== 'gemini'
        ) {
          return '-';
        }
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
      title: t('创建时间'),
      dataIndex: 'created_time',
      render: (timestamp) => renderTimestamp(timestamp),
    },
    {
      title: t('优先级'),
      dataIndex: 'priority',
      width: 100,
      render: (value) => value ?? 0,
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
