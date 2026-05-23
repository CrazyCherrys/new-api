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
import {
  Modal,
  Form,
  Button,
  Space,
  Switch,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../../helpers';

const DEFAULT_IMAGE_CAPABILITIES = ['image_generation', 'image_editing'];
const DEFAULT_VIDEO_CAPABILITIES = ['image_to_video', 'text_to_video'];
const DEFAULT_VIDEO_DURATIONS = [5, 10];
const IMAGE_CAPABILITY_EDITING = 'image_editing';
const DEFAULT_REFERENCE_IMAGE_LIMIT = 1;
const MAX_REFERENCE_IMAGE_LIMIT = 20;
const OPENAI_IMAGE_SIZE_ENDPOINTS = new Set(['openai', 'openai-response']);

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

const EditModelMappingModal = ({
  visible,
  handleClose,
  editingMapping,
  refresh,
}) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [formApi, setFormApi] = useState(null);
  const [selectedModelType, setSelectedModelType] = useState(1);
  const [selectedRequestEndpoint, setSelectedRequestEndpoint] =
    useState('openai');
  const [referenceImageLimitEnabled, setReferenceImageLimitEnabled] =
    useState(false);
  const [selectedImageCapabilities, setSelectedImageCapabilities] = useState([]);
  const isImageModel = Number(selectedModelType) === 2;
  const isVideoModel = Number(selectedModelType) === 3;
  const canConfigureReferenceImageLimit =
    isImageModel && selectedImageCapabilities.includes(IMAGE_CAPABILITY_EDITING);
  const usesOpenAIImageEndpoint =
    isImageModel && OPENAI_IMAGE_SIZE_ENDPOINTS.has(selectedRequestEndpoint);
  const canConfigureImageResolution =
    isImageModel && !usesOpenAIImageEndpoint;

  const modelSeriesOptions = [
    { value: 'openai', label: 'OpenAI' },
    { value: 'anthropic', label: 'Anthropic (Claude)' },
    { value: 'google', label: 'Google (Gemini)' },
    { value: 'azure', label: 'Azure OpenAI' },
    { value: 'aws', label: 'AWS Bedrock' },
    { value: 'cohere', label: 'Cohere' },
    { value: 'mistral', label: 'Mistral AI' },
    { value: 'deepseek', label: 'DeepSeek' },
    { value: 'zhipu', label: '智谱AI' },
    { value: 'baidu', label: '百度文心' },
    { value: 'alibaba', label: '阿里通义' },
    { value: 'tencent', label: '腾讯混元' },
    { value: 'moonshot', label: 'Moonshot (Kimi)' },
    { value: 'minimax', label: 'MiniMax' },
    { value: 'doubao', label: '豆包' },
    { value: 'other', label: t('其他') },
  ];

  const modelTypeOptions = [
    { value: 1, label: t('对话') },
    { value: 2, label: t('绘画') },
    { value: 3, label: t('视频') },
    { value: 4, label: t('音频') },
  ];

  const chatEndpointOptions = [
    { value: 'openai', label: 'OpenAI (/v1/chat/completions)' },
    { value: 'anthropic', label: 'Claude (/v1/messages)' },
    {
      value: 'gemini',
      label: 'Gemini (/v1beta/models/{model}:generateContent)',
    },
  ];

  const imageEndpointOptions = [
    { value: 'openai', label: 'OpenAI (/v1/images)' },
    { value: 'openai-response', label: 'OpenAI (/v1/responses)' },
    { value: 'gemini', label: 'Gemini 图片生成' },
  ];

  const videoEndpointOptions = [
    {
      value: 'openai-video-generation',
      label: 'OpenAI Video Generations (/v1/video/generations)',
    },
    {
      value: 'openai-video',
      label: 'OpenAI Videos (Sora, /v1/videos)',
    },
  ];

  const getRequestEndpointOptions = (modelType) => {
    switch (Number(modelType)) {
      case 1:
        return chatEndpointOptions;
      case 2:
        return imageEndpointOptions;
      case 3:
        return videoEndpointOptions;
      default:
        return chatEndpointOptions;
    }
  };

  const getDefaultRequestEndpoint = (modelType) => {
    const options = getRequestEndpointOptions(modelType);
    return options[0]?.value || 'openai';
  };

  const isValidRequestEndpointForModelType = (modelType, endpoint) => {
    const normalizedEndpoint = normalizeRequestEndpoint(endpoint);
    return getRequestEndpointOptions(modelType).some(
      (option) => option.value === normalizedEndpoint,
    );
  };

  const requestEndpointOptions = getRequestEndpointOptions(selectedModelType);

  const imageResolutionOptions = [
    { value: '1K', label: '1K' },
    { value: '2K', label: '2K' },
    { value: '4K', label: '4K' },
  ];

  const aspectRatioOptions = [
    { value: '1:1', label: '1:1' },
    { value: '16:9', label: '16:9' },
    { value: '9:16', label: '9:16' },
    { value: '4:3', label: '4:3' },
    { value: '3:4', label: '3:4' },
    { value: '4:5', label: '4:5' },
    { value: '5:4', label: '5:4' },
    { value: '3:2', label: '3:2' },
    { value: '2:3', label: '2:3' },
    { value: '21:9', label: '21:9' },
    { value: 'auto', label: 'Auto' },
  ];

  const imageCapabilityOptions = [
    { value: 'image_generation', label: t('图片生成') },
    { value: 'image_editing', label: t('图像编辑') },
  ];

  const videoCapabilityOptions = [
    { value: 'image_to_video', label: t('图生视频') },
    { value: 'text_to_video', label: t('文生视频') },
  ];

  const parseJsonArray = (value) => {
    if (Array.isArray(value)) {
      return value;
    }
    if (typeof value !== 'string' || value.trim() === '') {
      return [];
    }
    try {
      const parsed = JSON.parse(value);
      return Array.isArray(parsed) ? parsed : [];
    } catch (e) {
      return [];
    }
  };

  const normalizeDurationValue = (value) => {
    const raw = String(value ?? '')
      .trim()
      .toLowerCase();
    if (!raw) {
      return null;
    }
    let normalized = raw;
    if (normalized.endsWith('秒')) {
      normalized = normalized.slice(0, -1).trim();
    }
    if (normalized.endsWith('s')) {
      normalized = normalized.slice(0, -1).trim();
    }
    if (!/^\d+$/.test(normalized)) {
      return null;
    }
    const seconds = Number(normalized);
    return Number.isInteger(seconds) && seconds > 0 ? seconds : null;
  };

  const normalizeDurationOptions = (values) => {
    const source = Array.isArray(values) ? values : [];
    const seen = new Set();
    const normalized = [];
    source.forEach((item) => {
      String(item ?? '')
        .split(/[,\s，、]+/)
        .forEach((part) => {
          const seconds = normalizeDurationValue(part);
          if (!seconds || seen.has(seconds)) {
            return;
          }
          seen.add(seconds);
          normalized.push(seconds);
        });
    });
    return normalized;
  };

  const formatDurationTags = (values) =>
    normalizeDurationOptions(values).map((item) => `${item}s`);

  useEffect(() => {
    if (visible && formApi) {
      if (editingMapping) {
        // 解析 JSON 字符串为数组
        const resolutions = parseJsonArray(editingMapping.resolutions);
        const aspectRatios = parseJsonArray(editingMapping.aspect_ratios);
        let imageCapabilities = parseJsonArray(
          editingMapping.image_capabilities,
        );
        let videoCapabilities = parseJsonArray(
          editingMapping.video_capabilities,
        );
        let durationValues = parseJsonArray(editingMapping.duration_options);
        if (Number(editingMapping.model_type) === 2 && imageCapabilities.length === 0) {
          imageCapabilities = imageCapabilityOptions.map((item) => item.value);
        }
        if (Number(editingMapping.model_type) === 3 && videoCapabilities.length === 0) {
          videoCapabilities = videoCapabilityOptions
            .filter((item) => DEFAULT_VIDEO_CAPABILITIES.includes(item.value))
            .map((item) => item.value);
        }
        if (Number(editingMapping.model_type) === 3 && durationValues.length === 0) {
          durationValues = DEFAULT_VIDEO_DURATIONS;
        }

        const nextModelType = Number(editingMapping.model_type) || 1;
        const normalizedEndpoint =
          normalizeRequestEndpoint(editingMapping.request_endpoint) ||
          getDefaultRequestEndpoint(nextModelType);
        const nextEndpoint = isValidRequestEndpointForModelType(
          nextModelType,
          normalizedEndpoint,
        )
          ? normalizedEndpoint
          : getDefaultRequestEndpoint(nextModelType);

        const isImageMapping = nextModelType === 2;
        const shouldLoadResolutions =
          isImageMapping && !OPENAI_IMAGE_SIZE_ENDPOINTS.has(nextEndpoint);
        setSelectedModelType(nextModelType);
        setSelectedRequestEndpoint(nextEndpoint);
        setSelectedImageCapabilities(imageCapabilities);
        setReferenceImageLimitEnabled(
          nextModelType === 2 &&
            imageCapabilities.includes(IMAGE_CAPABILITY_EDITING) &&
            Number(editingMapping.reference_image_limit) > 0,
        );

        formApi.setValues({
          ...editingMapping,
          request_endpoint: nextEndpoint,
          actual_model:
            editingMapping.actual_model || editingMapping.request_model || '',
          resolutions: shouldLoadResolutions ? resolutions : [],
          aspect_ratios: isImageMapping ? aspectRatios : [],
          image_capabilities: imageCapabilities,
          reference_image_limit:
            Number(editingMapping.reference_image_limit) > 0
              ? Number(editingMapping.reference_image_limit)
              : DEFAULT_REFERENCE_IMAGE_LIMIT,
          video_capabilities: videoCapabilities,
          duration_options: formatDurationTags(durationValues),
          status: editingMapping.status === 1,
          priority: editingMapping.priority ?? 0,
        });
      } else {
        setSelectedModelType(1);
        setSelectedRequestEndpoint('openai');
        setSelectedImageCapabilities([]);
        setReferenceImageLimitEnabled(false);

        formApi.setValues({
          request_model: '',
          actual_model: '',
          display_name: '',
          model_series: '',
          model_type: 1,
          status: true,
          priority: 0,
          request_endpoint: 'openai',
          resolutions: [],
          aspect_ratios: [],
          image_capabilities: [],
          reference_image_limit: DEFAULT_REFERENCE_IMAGE_LIMIT,
          video_capabilities: [],
          duration_options: [],
        });
      }
    }
  }, [visible, editingMapping, formApi]);

  useEffect(() => {
    if (!visible || !formApi || !isImageModel) {
      return;
    }

    const currentCapabilities = formApi.getValue('image_capabilities');
    if (Array.isArray(currentCapabilities) && currentCapabilities.length > 0) {
      return;
    }

    formApi.setValue('image_capabilities', DEFAULT_IMAGE_CAPABILITIES);
    setSelectedImageCapabilities(DEFAULT_IMAGE_CAPABILITIES);
  }, [visible, formApi, selectedModelType]);

  useEffect(() => {
    if (!visible || !formApi || !isVideoModel) {
      return;
    }

    const currentCapabilities = formApi.getValue('video_capabilities');
    if (Array.isArray(currentCapabilities) && currentCapabilities.length > 0) {
      return;
    }

    formApi.setValue('video_capabilities', DEFAULT_VIDEO_CAPABILITIES);
  }, [visible, formApi, isVideoModel]);

  const handleSubmit = async (values) => {
    if (
      Number(values.model_type) === 2 &&
      (!Array.isArray(values.image_capabilities) ||
        values.image_capabilities.length === 0)
    ) {
      showError(t('请选择至少一个模型能力'));
      return;
    }
    if (
      Number(values.model_type) === 3 &&
      (!Array.isArray(values.video_capabilities) ||
        values.video_capabilities.length === 0)
    ) {
      showError(t('请选择至少一个视频能力'));
      return;
    }
    const normalizedDurations = normalizeDurationOptions(
      values.duration_options,
    );
    if (Number(values.model_type) === 3 && normalizedDurations.length === 0) {
      showError(t('请至少输入一个时长选项'));
      return;
    }

    const imageCapabilities = Array.isArray(values.image_capabilities)
      ? values.image_capabilities
      : [];
    const shouldSubmitReferenceImageLimit =
      Number(values.model_type) === 2 &&
      imageCapabilities.includes(IMAGE_CAPABILITY_EDITING) &&
      referenceImageLimitEnabled;
    if (
      shouldSubmitReferenceImageLimit &&
      (!Number.isFinite(Number(values.reference_image_limit)) ||
        Number(values.reference_image_limit) < 1)
    ) {
      showError(t('请输入有效的参考图张数限制'));
      return;
    }

    setLoading(true);
    try {
      const modelType = Number(values.model_type);
      const requestEndpoint = normalizeRequestEndpoint(values.request_endpoint);
      const shouldSubmitImageSettings = modelType === 2;
      const payload = {
        ...values,
        request_endpoint: requestEndpoint,
        actual_model:
          typeof values.actual_model === 'string'
            ? values.actual_model.trim()
            : values.actual_model,
        status: values.status ? 1 : 0,
        priority: Number.isFinite(Number(values.priority))
          ? Number(values.priority)
          : 0,
        // 将数组转换为 JSON 字符串
        resolutions:
          shouldSubmitImageSettings &&
          !OPENAI_IMAGE_SIZE_ENDPOINTS.has(requestEndpoint) &&
          values.resolutions
          ? JSON.stringify(values.resolutions)
          : '',
        aspect_ratios:
          shouldSubmitImageSettings && values.aspect_ratios
          ? JSON.stringify(values.aspect_ratios)
          : '',
        image_capabilities:
          modelType === 2 && values.image_capabilities
            ? JSON.stringify(values.image_capabilities)
            : '',
        reference_image_limit: shouldSubmitReferenceImageLimit
          ? Math.min(
              MAX_REFERENCE_IMAGE_LIMIT,
              Math.max(1, Math.floor(Number(values.reference_image_limit))),
            )
          : 0,
        video_capabilities:
          modelType === 3 && values.video_capabilities
            ? JSON.stringify(values.video_capabilities)
            : '',
        duration_options:
          modelType === 3
            ? JSON.stringify(normalizedDurations)
            : '',
      };

      if (editingMapping) {
        payload.id = editingMapping.id;
      }

      const url = editingMapping
        ? '/api/model-mapping/'
        : '/api/model-mapping/';
      const method = editingMapping ? 'put' : 'post';

      const res = await API[method](url, payload);
      const { success, message } = res.data;

      if (success) {
        showSuccess(editingMapping ? t('更新成功') : t('创建成功'));
        handleClose();
        refresh();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  };

  const handleSelectAllAspectRatios = () => {
    const allValues = aspectRatioOptions.map((opt) => opt.value);
    formApi?.setValue('aspect_ratios', allValues);
  };

  const handleDeselectAllAspectRatios = () => {
    formApi?.setValue('aspect_ratios', []);
  };

  const handleSelectAllResolutions = () => {
    const allValues = imageResolutionOptions.map((opt) => opt.value);
    formApi?.setValue('resolutions', allValues);
  };

  const handleDeselectAllResolutions = () => {
    formApi?.setValue('resolutions', []);
  };

  return (
    <Modal
      title={editingMapping ? t('编辑模型设置') : t('添加模型设置')}
      visible={visible}
      onCancel={handleClose}
      footer={null}
      width={600}
    >
      <Form
        getFormApi={(api) => setFormApi(api)}
        onSubmit={handleSubmit}
        labelPosition='left'
        labelWidth={120}
      >
        <Form.Input
          field='request_model'
          label={t('模型ID')}
          placeholder={t('用户请求时使用的模型ID')}
          rules={[{ required: true, message: t('请输入模型ID') }]}
          disabled={!!editingMapping}
        />
        <Form.Input
          field='actual_model'
          label={t('实际调用模型ID')}
          placeholder={t('为空时默认与模型ID相同')}
        />
        <Form.Input
          field='display_name'
          label={t('显示名称')}
          placeholder={t('用户界面显示的友好名称')}
          rules={[{ required: true, message: t('请输入显示名称') }]}
        />
        <Form.Select
          field='model_series'
          label={t('模型系列')}
          placeholder={t('选择模型系列/厂商')}
          optionList={modelSeriesOptions}
          filter
        />
        <Form.Select
          field='model_type'
          label={t('模型类型')}
          placeholder={t('选择模型类型')}
          optionList={modelTypeOptions}
          rules={[{ required: true, message: t('请选择模型类型') }]}
          onChange={(value) => {
            const nextModelType = Number(value) || 1;
            setSelectedModelType(nextModelType);
            const currentEndpoint = normalizeRequestEndpoint(
              formApi?.getValue('request_endpoint'),
            );
            if (
              !currentEndpoint ||
              !isValidRequestEndpointForModelType(nextModelType, currentEndpoint)
            ) {
              const nextEndpoint = getDefaultRequestEndpoint(nextModelType);
              formApi?.setValue('request_endpoint', nextEndpoint);
              setSelectedRequestEndpoint(nextEndpoint);
              if (OPENAI_IMAGE_SIZE_ENDPOINTS.has(nextEndpoint)) {
                formApi?.setValue('resolutions', []);
              }
            } else {
              setSelectedRequestEndpoint(currentEndpoint);
              if (OPENAI_IMAGE_SIZE_ENDPOINTS.has(currentEndpoint)) {
                formApi?.setValue('resolutions', []);
              }
            }

            if (nextModelType === 2) {
              formApi?.setValue('video_capabilities', []);
              formApi?.setValue('duration_options', []);
              const currentCapabilities =
                formApi?.getValue('image_capabilities');
              if (
                !Array.isArray(currentCapabilities) ||
                currentCapabilities.length === 0
              ) {
                formApi?.setValue(
                  'image_capabilities',
                  DEFAULT_IMAGE_CAPABILITIES,
                );
                setSelectedImageCapabilities(DEFAULT_IMAGE_CAPABILITIES);
              }
            } else if (nextModelType === 3) {
              formApi?.setValue('image_capabilities', []);
              setSelectedImageCapabilities([]);
              formApi?.setValue(
                'reference_image_limit',
                DEFAULT_REFERENCE_IMAGE_LIMIT,
              );
              setReferenceImageLimitEnabled(false);
              formApi?.setValue('resolutions', []);
              formApi?.setValue('aspect_ratios', []);
              const currentVideoCapabilities =
                formApi?.getValue('video_capabilities');
              if (
                !Array.isArray(currentVideoCapabilities) ||
                currentVideoCapabilities.length === 0
              ) {
                formApi?.setValue(
                  'video_capabilities',
                  DEFAULT_VIDEO_CAPABILITIES,
                );
              }
              const currentDurations = formApi?.getValue('duration_options');
              if (
                !Array.isArray(currentDurations) ||
                currentDurations.length === 0
              ) {
                formApi?.setValue(
                  'duration_options',
                  formatDurationTags(DEFAULT_VIDEO_DURATIONS),
                );
              }
            } else {
              formApi?.setValue('image_capabilities', []);
              setSelectedImageCapabilities([]);
              formApi?.setValue(
                'reference_image_limit',
                DEFAULT_REFERENCE_IMAGE_LIMIT,
              );
              setReferenceImageLimitEnabled(false);
              formApi?.setValue('video_capabilities', []);
              formApi?.setValue('duration_options', []);
              formApi?.setValue('resolutions', []);
              formApi?.setValue('aspect_ratios', []);
            }
          }}
        />
        <Form.Select
          field='request_endpoint'
          label={t('请求端点')}
          placeholder={t('选择请求端点类型')}
          optionList={requestEndpointOptions}
          rules={[{ required: true, message: t('请选择请求端点') }]}
          onChange={(value) => {
            const nextEndpoint = normalizeRequestEndpoint(value);
            formApi?.setValue('request_endpoint', nextEndpoint);
            setSelectedRequestEndpoint(nextEndpoint);
            if (OPENAI_IMAGE_SIZE_ENDPOINTS.has(nextEndpoint)) {
              formApi?.setValue('resolutions', []);
            }
          }}
        />
        <Form.Switch field='status' label={t('状态')} size='large' />
        <Form.InputNumber
          field='priority'
          label={t('优先级')}
          min={0}
          precision={0}
          style={{ width: '100%' }}
        />
        <div hidden={!isImageModel}>
          <Form.CheckboxGroup
            field='image_capabilities'
            label={t('模型能力')}
            options={imageCapabilityOptions}
            direction='horizontal'
            onChange={(value) => {
              const nextCapabilities = Array.isArray(value) ? value : [];
              setSelectedImageCapabilities(nextCapabilities);
              if (!nextCapabilities.includes(IMAGE_CAPABILITY_EDITING)) {
                setReferenceImageLimitEnabled(false);
              }
            }}
            rules={
              isImageModel
                ? [
                    {
                      required: true,
                      message: t('请选择至少一个模型能力'),
                    },
                  ]
                : []
            }
          />
        </div>
        {canConfigureReferenceImageLimit && (
          <>
            <Form.Slot label={t('限制参考图张数')}>
              <Switch
                size='large'
                checked={referenceImageLimitEnabled}
                onChange={(checked) => {
                  setReferenceImageLimitEnabled(checked);
                  if (
                    checked &&
                    !Number(formApi?.getValue('reference_image_limit'))
                  ) {
                    formApi?.setValue(
                      'reference_image_limit',
                      DEFAULT_REFERENCE_IMAGE_LIMIT,
                    );
                  }
                }}
              />
              <div
                style={{
                  marginTop: 6,
                  fontSize: 12,
                  color: 'var(--semi-color-text-2)',
                }}
              >
                {t('不勾选则不限制')}
              </div>
            </Form.Slot>
            {referenceImageLimitEnabled && (
              <Form.InputNumber
                field='reference_image_limit'
                label={t('最多参考图张数')}
                min={1}
                max={MAX_REFERENCE_IMAGE_LIMIT}
                precision={0}
                style={{ width: '100%' }}
              />
            )}
          </>
        )}
        <div hidden={!isVideoModel}>
          <Form.CheckboxGroup
            field='video_capabilities'
            label={t('视频能力')}
            options={videoCapabilityOptions}
            direction='horizontal'
            rules={
              isVideoModel
                ? [
                    {
                      required: true,
                      message: t('请选择至少一个视频能力'),
                    },
                  ]
                : []
            }
          />
        </div>
        <div hidden={!isVideoModel}>
          <Form.TagInput
            field='duration_options'
            label={t('时长选项')}
            placeholder={t('输入时长，如 5s 或 10s，回车添加')}
            addOnBlur
            showClear
            style={{ width: '100%' }}
            onChange={(values) => {
              formApi?.setValue('duration_options', formatDurationTags(values));
            }}
            rules={
              isVideoModel
                ? [
                    {
                      required: true,
                      message: t('请至少输入一个时长选项'),
                    },
                  ]
                : []
            }
          />
        </div>
        {canConfigureImageResolution && (
          <div>
            <div
              style={{
                marginBottom: 8,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <span style={{ fontSize: 14, fontWeight: 600 }}>
                {t('分辨率')}
              </span>
              <Space>
                <Button size='small' onClick={handleSelectAllResolutions}>
                  {t('全选')}
                </Button>
                <Button size='small' onClick={handleDeselectAllResolutions}>
                  {t('取消全选')}
                </Button>
              </Space>
            </div>
            <Form.CheckboxGroup
              field='resolutions'
              options={imageResolutionOptions}
              direction='horizontal'
            />
          </div>
        )}
        {isImageModel && (
          <div>
            <div
              style={{
                marginBottom: 8,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <span style={{ fontSize: 14, fontWeight: 600 }}>
                {t('宽高比')}
              </span>
              <Space>
                <Button size='small' onClick={handleSelectAllAspectRatios}>
                  {t('全选')}
                </Button>
                <Button size='small' onClick={handleDeselectAllAspectRatios}>
                  {t('取消全选')}
                </Button>
              </Space>
            </div>
            <Form.CheckboxGroup
              field='aspect_ratios'
              options={aspectRatioOptions}
              direction='horizontal'
            />
          </div>
        )}
        <Space
          style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-end' }}
        >
          <Button onClick={handleClose}>{t('取消')}</Button>
          <Button
            theme='solid'
            type='primary'
            htmlType='submit'
            loading={loading}
          >
            {t('提交')}
          </Button>
        </Space>
      </Form>
    </Modal>
  );
};

export default EditModelMappingModal;
