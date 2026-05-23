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
  InputNumber,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../../helpers';

const DEFAULT_IMAGE_CAPABILITIES = ['image_generation', 'image_editing'];
const DEFAULT_VIDEO_CAPABILITIES = ['image_to_video'];

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
  const [selectedResolutions, setSelectedResolutions] = useState([]);
  const [selectedAspectRatios, setSelectedAspectRatios] = useState([]);
  const [selectedModelType, setSelectedModelType] = useState(1);
  const [selectedRequestEndpoint, setSelectedRequestEndpoint] =
    useState('openai');
  const isImageModel = Number(selectedModelType) === 2;
  const isVideoModel = Number(selectedModelType) === 3;
  const isGeminiImageModel =
    isImageModel && selectedRequestEndpoint === 'gemini';
  const showResolutionAndAspectRatio = isGeminiImageModel || isVideoModel;

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
    { value: 'openai_mod', label: 'OpenAI魔改' },
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

  const videoResolutionOptions = [
    { value: '1280x720', label: '1280x720' },
    { value: '720x1280', label: '720x1280' },
    { value: '1920x1080', label: '1920x1080' },
    { value: '1080x1920', label: '1080x1920' },
    { value: '1024x1024', label: '1024x1024' },
    { value: '960x960', label: '960x960' },
    { value: '1792x1024', label: '1792x1024' },
    { value: '1024x1792', label: '1024x1792' },
  ];

  const resolutionOptions = isVideoModel
    ? videoResolutionOptions
    : imageResolutionOptions;

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
  ];

  const durationOptions = [
    { value: 5, label: '5s' },
    { value: 10, label: '10s' },
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

        setSelectedResolutions(resolutions);
        setSelectedAspectRatios(aspectRatios);
        setSelectedModelType(nextModelType);
        setSelectedRequestEndpoint(nextEndpoint);

        formApi.setValues({
          ...editingMapping,
          request_endpoint: nextEndpoint,
          actual_model:
            editingMapping.actual_model || editingMapping.request_model || '',
          resolutions,
          aspect_ratios: aspectRatios,
          image_capabilities: imageCapabilities,
          video_capabilities: videoCapabilities,
          duration_options: durationValues,
          status: editingMapping.status === 1,
          priority: editingMapping.priority ?? 0,
        });
      } else {
        setSelectedResolutions([]);
        setSelectedAspectRatios([]);
        setSelectedModelType(1);
        setSelectedRequestEndpoint('openai');

        formApi.setValues({
          request_model: '',
          actual_model: '',
          display_name: '',
          model_series: '',
          model_type: 1,
          description: '',
          status: true,
          priority: 0,
          request_endpoint: 'openai',
          resolutions: [],
          aspect_ratios: [],
          image_capabilities: [],
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
    if (
      Number(values.model_type) === 3 &&
      (!Array.isArray(values.duration_options) ||
        values.duration_options.length === 0)
    ) {
      showError(t('请至少选择一个时长选项'));
      return;
    }

    setLoading(true);
    try {
      const modelType = Number(values.model_type);
      const requestEndpoint = normalizeRequestEndpoint(values.request_endpoint);
      const shouldSubmitImageSettings =
        modelType === 2 && requestEndpoint === 'gemini';
      const shouldSubmitVideoSettings = modelType === 3;
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
          (shouldSubmitImageSettings || shouldSubmitVideoSettings) &&
          values.resolutions
          ? JSON.stringify(values.resolutions)
          : '',
        aspect_ratios:
          (shouldSubmitImageSettings || shouldSubmitVideoSettings) &&
          values.aspect_ratios
          ? JSON.stringify(values.aspect_ratios)
          : '',
        image_capabilities:
          modelType === 2 && values.image_capabilities
            ? JSON.stringify(values.image_capabilities)
            : '',
        video_capabilities:
          modelType === 3 && values.video_capabilities
            ? JSON.stringify(values.video_capabilities)
            : '',
        duration_options:
          modelType === 3 && values.duration_options
            ? JSON.stringify(values.duration_options.map(Number))
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
    setSelectedAspectRatios(allValues);
    formApi?.setValue('aspect_ratios', allValues);
  };

  const handleDeselectAllAspectRatios = () => {
    setSelectedAspectRatios([]);
    formApi?.setValue('aspect_ratios', []);
  };

  const handleSelectAllResolutions = () => {
    const allValues = resolutionOptions.map((opt) => opt.value);
    setSelectedResolutions(allValues);
    formApi?.setValue('resolutions', allValues);
  };

  const handleDeselectAllResolutions = () => {
    setSelectedResolutions([]);
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
            const previousModelType = Number(selectedModelType) || 1;
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
            } else {
              setSelectedRequestEndpoint(currentEndpoint);
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
              }
              const nextEndpoint =
                formApi?.getValue('request_endpoint') ||
                getDefaultRequestEndpoint(nextModelType);
              if (nextEndpoint !== 'gemini' || previousModelType !== 2) {
                formApi?.setValue('resolutions', []);
                formApi?.setValue('aspect_ratios', []);
                setSelectedResolutions([]);
                setSelectedAspectRatios([]);
              }
            } else if (nextModelType === 3) {
              formApi?.setValue('image_capabilities', []);
              if (previousModelType !== 3) {
                formApi?.setValue('resolutions', []);
                formApi?.setValue('aspect_ratios', []);
                setSelectedResolutions([]);
                setSelectedAspectRatios([]);
              }
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
            } else {
              formApi?.setValue('image_capabilities', []);
              formApi?.setValue('video_capabilities', []);
              formApi?.setValue('duration_options', []);
              formApi?.setValue('resolutions', []);
              formApi?.setValue('aspect_ratios', []);
              setSelectedResolutions([]);
              setSelectedAspectRatios([]);
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
            setSelectedRequestEndpoint(nextEndpoint);
            formApi?.setValue('request_endpoint', nextEndpoint);
            if (isImageModel && nextEndpoint !== 'gemini') {
              formApi?.setValue('resolutions', []);
              formApi?.setValue('aspect_ratios', []);
              setSelectedResolutions([]);
              setSelectedAspectRatios([]);
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
          <Form.CheckboxGroup
            field='duration_options'
            label={t('时长选项')}
            options={durationOptions}
            direction='horizontal'
            rules={
              isVideoModel
                ? [
                    {
                      required: true,
                      message: t('请至少选择一个时长选项'),
                    },
                  ]
                : []
            }
          />
        </div>
        {showResolutionAndAspectRatio && (
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
              options={resolutionOptions}
              direction='horizontal'
            />
          </div>
        )}
        {showResolutionAndAspectRatio && (
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
        <Form.TextArea
          field='description'
          label={t('描述')}
          placeholder={t('输入模型描述')}
          autosize
          showClear
        />
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
