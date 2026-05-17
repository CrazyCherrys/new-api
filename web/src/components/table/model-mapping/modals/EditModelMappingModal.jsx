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
  Select,
  Button,
  Space,
  InputNumber,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../../helpers';

const DEFAULT_IMAGE_CAPABILITIES = ['image_generation', 'image_editing'];
const DEFAULT_MODEL_ENDPOINTS = {
  1: 'openai',
  2: 'openai',
  3: 'openai-video-generation',
  4: 'openai',
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
  const isImageModel = Number(selectedModelType) === 2;

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

  const requestEndpointOptions =
    selectedModelType === 2
      ? [
          { value: 'openai', label: 'OpenAI (/v1/images)' },
          { value: 'openai-response', label: 'OpenAI (/v1/responses)' },
          { value: 'gemini', label: 'Gemini' },
          { value: 'openai_mod', label: 'OpenAI魔改' },
        ]
      : selectedModelType === 3
        ? [
            {
              value: 'openai-video-generation',
              label: 'OpenAI Video Generations (/v1/video/generations)',
            },
            {
              value: 'openai-video',
              label: 'OpenAI Videos (Sora, /v1/videos)',
            },
          ]
        : [
          { value: 'openai', label: 'OpenAI (/v1/images)' },
          { value: 'gemini', label: 'Gemini' },
          { value: 'openai_mod', label: 'OpenAI魔改' },
        ];

  const resolutionOptions = [
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
        if (Number(editingMapping.model_type) === 2 && imageCapabilities.length === 0) {
          imageCapabilities = imageCapabilityOptions.map((item) => item.value);
        }

        setSelectedResolutions(resolutions);
        setSelectedAspectRatios(aspectRatios);
        setSelectedModelType(Number(editingMapping.model_type) || 1);

        formApi.setValues({
          ...editingMapping,
          actual_model:
            editingMapping.actual_model || editingMapping.request_model || '',
          resolutions,
          aspect_ratios: aspectRatios,
          image_capabilities: imageCapabilities,
          status: editingMapping.status === 1,
          priority: editingMapping.priority ?? 0,
        });
      } else {
        setSelectedResolutions([]);
        setSelectedAspectRatios([]);
        setSelectedModelType(1);

        formApi.setValues({
          request_model: '',
          actual_model: '',
          display_name: '',
          model_series: '',
          model_type: 1,
          description: '',
          status: true,
          priority: 0,
          request_endpoint: DEFAULT_MODEL_ENDPOINTS[1],
          resolutions: [],
          aspect_ratios: [],
          image_capabilities: [],
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

  const handleSubmit = async (values) => {
    if (
      Number(values.model_type) === 2 &&
      (!Array.isArray(values.image_capabilities) ||
        values.image_capabilities.length === 0)
    ) {
      showError(t('请选择至少一个模型能力'));
      return;
    }

    setLoading(true);
    try {
      const payload = {
        ...values,
        actual_model:
          typeof values.actual_model === 'string'
            ? values.actual_model.trim()
            : values.actual_model,
        status: values.status ? 1 : 0,
        priority: Number.isFinite(Number(values.priority))
          ? Number(values.priority)
          : 0,
        // 将数组转换为 JSON 字符串
        resolutions: values.resolutions
          ? JSON.stringify(values.resolutions)
          : '',
        aspect_ratios: values.aspect_ratios
          ? JSON.stringify(values.aspect_ratios)
          : '',
        image_capabilities:
          Number(values.model_type) === 2 && values.image_capabilities
            ? JSON.stringify(values.image_capabilities)
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
            setSelectedModelType(Number(value) || 1);
            const currentEndpoint = formApi?.getValue('request_endpoint');
            if (Number(value) === 2) {
              if (
                !['openai', 'openai-response', 'gemini', 'openai_mod'].includes(
                  currentEndpoint,
                )
              ) {
                formApi?.setValue('request_endpoint', DEFAULT_MODEL_ENDPOINTS[2]);
              }
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
            } else if (Number(value) === 3) {
              if (
                !['openai-video-generation', 'openai-video'].includes(
                  currentEndpoint,
                )
              ) {
                formApi?.setValue('request_endpoint', DEFAULT_MODEL_ENDPOINTS[3]);
              }
            } else {
              if (
                !['openai', 'gemini', 'openai_mod'].includes(currentEndpoint)
              ) {
                formApi?.setValue(
                  'request_endpoint',
                  DEFAULT_MODEL_ENDPOINTS[Number(value)] || 'openai',
                );
              }
              formApi?.setValue('image_capabilities', []);
            }
          }}
        />
        <Form.Select
          field='request_endpoint'
          label={t('请求端点')}
          placeholder={t('选择请求端点类型')}
          optionList={requestEndpointOptions}
          rules={[{ required: true, message: t('请选择请求端点') }]}
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
        <div>
          <div
            style={{
              marginBottom: 8,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <span style={{ fontSize: 14, fontWeight: 600 }}>{t('分辨率')}</span>
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
        <div>
          <div
            style={{
              marginBottom: 8,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <span style={{ fontSize: 14, fontWeight: 600 }}>{t('宽高比')}</span>
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
