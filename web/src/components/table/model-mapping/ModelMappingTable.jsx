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
  Typography,
  Popconfirm,
  Button,
  Space,
  Switch,
} from '@douyinfe/semi-ui';
import { IconEdit, IconDelete } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, showWarning } from '../../../helpers';
import {
  CHAT_CAPABILITY_FILE_UPLOAD,
  CHAT_CAPABILITY_IMAGE_UPLOAD,
  CHAT_CAPABILITY_WEB_SEARCH,
  normalizeCanvasChatCapabilities,
} from '../../../helpers/canvasChat';
import {
  formatModelSeriesLabel,
  ModelSeriesIcon,
} from '../../../helpers/modelSeries';

const { Text } = Typography;
const DEFAULT_IMAGE_CAPABILITIES = ['image_generation', 'image_editing'];
const DEFAULT_VIDEO_CAPABILITIES = ['image_to_video'];
const DEFAULT_VIDEO_DURATIONS = [5, 10];
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
  activeModelType,
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

  const normalizeVideoCapabilities = (raw) => {
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
        return [...DEFAULT_VIDEO_CAPABILITIES];
      }
    }
    return [...DEFAULT_VIDEO_CAPABILITIES];
  };

  const normalizeDurationOptions = (raw) => {
    const source = Array.isArray(raw)
      ? raw
      : (() => {
          if (typeof raw !== 'string' || raw.trim() === '') {
            return [];
          }
          try {
            const parsed = JSON.parse(raw);
            return Array.isArray(parsed) ? parsed : [];
          } catch (e) {
            return [];
          }
        })();
    const seen = new Set();
    const normalized = [];
    source.forEach((item) => {
      const seconds = Number(
        String(item ?? '')
          .trim()
          .toLowerCase()
          .replace(/秒$/, '')
          .replace(/s$/, '')
          .trim(),
      );
      if (!Number.isInteger(seconds) || seconds <= 0 || seen.has(seconds)) {
        return;
      }
      seen.add(seconds);
      normalized.push(seconds);
    });
    return normalized;
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
      1: ['openai', 'openai-response', 'anthropic', 'gemini'],
      2: ['openai', 'openai-response', 'gemini'],
      3: ['openai-video-generation', 'openai-video'],
    };
    if (!endpointOptions[Number(modelType)]) {
      return true;
    }
    return (endpointOptions[Number(modelType)] || []).includes(
      normalizedEndpoint,
    );
  };

  const sanitizeMappingPayloadForUpdate = (record) => {
    const payload = { ...record };
    const modelType = Number(payload.model_type);
    payload.request_endpoint = normalizeRequestEndpoint(
      payload.request_endpoint,
    );

    if (
      [1, 2, 3].includes(modelType) &&
      !isValidRequestEndpointForModelType(modelType, payload.request_endpoint)
    ) {
      payload.request_endpoint = getDefaultRequestEndpoint(modelType);
    }
    if (modelType !== 2) {
      payload.image_capabilities = '';
      payload.reference_image_limit = 0;
    }
    if (modelType !== 1) {
      payload.chat_capabilities = '';
    }
    if (modelType !== 3) {
      payload.video_capabilities = '';
      payload.duration_options = '';
    }
    if (modelType !== 2) {
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
    if (modelType === 1) {
      payload.chat_capabilities = JSON.stringify(
        normalizeCanvasChatCapabilities(payload.chat_capabilities),
      );
    }
    if (modelType === 3) {
      payload.video_capabilities = JSON.stringify(
        normalizeVideoCapabilities(payload.video_capabilities),
      );
      const durationOptions = normalizeDurationOptions(
        payload.duration_options,
      );
      payload.duration_options = JSON.stringify(
        durationOptions.length > 0 ? durationOptions : DEFAULT_VIDEO_DURATIONS,
      );
    }

    return payload;
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
        const warningMessages =
          res.data.data?.canvas_chat_diagnostic?.warning_messages || [];
        warningMessages.forEach((warningMessage) => {
          if (warningMessage) {
            showWarning(warningMessage);
          }
        });
        refresh();
      } else {
        showError(res.data.message || t('状态更新失败'));
      }
    } catch (error) {
      showError(error.message || t('状态更新失败'));
    }
  };

  const formatRequestEndpoint = (endpoint, modelType) => {
    const normalizedEndpoint = normalizeRequestEndpoint(endpoint);
    if (modelType === 1) {
      const endpointMap = {
        openai: 'OpenAI (/v1/chat/completions)',
        'openai-response': 'OpenAI Responses (/v1/responses)',
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

    if (modelType === 3) {
      const endpointMap = {
        openai: 'OpenAI',
        dalle: 'OpenAI',
        gemini: 'Gemini',
        'openai-video-generation':
          'OpenAI Video Generations (/v1/video/generations)',
        'openai-video': 'OpenAI Videos (Sora, /v1/videos)',
      };
      return endpointMap[normalizedEndpoint] || endpoint || '-';
    }

    return endpoint || '-';
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

  const formatChatCapabilities = (raw) => {
    const capabilities = normalizeCanvasChatCapabilities(raw);
    if (capabilities.length === 0) {
      return '-';
    }
    const capabilityMap = {
      [CHAT_CAPABILITY_IMAGE_UPLOAD]: t('图片上传'),
      [CHAT_CAPABILITY_FILE_UPLOAD]: t('文件上传'),
      [CHAT_CAPABILITY_WEB_SEARCH]: t('联网搜索'),
    };
    return capabilities
      .map((capability) => capabilityMap[capability] || capability)
      .join(', ');
  };

  const formatVideoCapabilities = (raw) => {
    const capabilities = normalizeVideoCapabilities(raw);
    const capabilityMap = {
      image_to_video: t('图生视频'),
      text_to_video: t('文生视频'),
    };
    return capabilities
      .map((capability) => capabilityMap[capability] || capability)
      .join(', ');
  };

  const formatDurationOptions = (raw) => {
    const durations = normalizeDurationOptions(raw);
    return durations.length > 0
      ? durations.map((item) => `${item}s`).join(', ')
      : '-';
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
      return t('不限制');
    }
    return t('最多 {{count}} 张', { count: limit });
  };

  const formatStringArrayField = (raw) => {
    if (!raw) {
      return '-';
    }
    try {
      const items = JSON.parse(raw);
      return Array.isArray(items) && items.length > 0 ? items.join(', ') : '-';
    } catch (e) {
      return '-';
    }
  };

  const baseColumns = [
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
      render: (text) =>
        text ? (
          <Space spacing={6}>
            <ModelSeriesIcon series={text} />
            <span>{formatModelSeriesLabel(text)}</span>
          </Space>
        ) : (
          '-'
        ),
    },
    {
      title: t('请求端点'),
      dataIndex: 'request_endpoint',
      render: (text, record) => formatRequestEndpoint(text, record.model_type),
    },
  ];

  const imageColumns = [
    {
      title: t('模型能力'),
      dataIndex: 'image_capabilities',
      render: (text) => formatImageCapabilities(text),
    },
    {
      title: t('参考图限制'),
      dataIndex: 'reference_image_limit',
      render: (_, record) => formatReferenceImageLimit(record),
    },
    {
      title: t('分辨率'),
      dataIndex: 'resolutions',
      render: (text) => formatStringArrayField(text),
    },
    {
      title: t('宽高比'),
      dataIndex: 'aspect_ratios',
      render: (text) => formatStringArrayField(text),
    },
  ];

  const chatColumns = [
    {
      title: t('对话能力'),
      dataIndex: 'chat_capabilities',
      render: (text) => formatChatCapabilities(text),
    },
  ];

  const videoColumns = [
    {
      title: t('视频能力'),
      dataIndex: 'video_capabilities',
      render: (text) => formatVideoCapabilities(text),
    },
    {
      title: t('时长选项'),
      dataIndex: 'duration_options',
      render: (text) => formatDurationOptions(text),
    },
  ];

  const trailingColumns = [
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

  const typeSpecificColumns =
    Number(activeModelType) === 1
      ? chatColumns
      : Number(activeModelType) === 2
        ? imageColumns
        : Number(activeModelType) === 3
          ? videoColumns
          : [];
  const columns = [...baseColumns, ...typeSpecificColumns, ...trailingColumns];

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
