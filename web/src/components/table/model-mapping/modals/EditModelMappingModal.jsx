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
  Input,
  Select,
  Button,
  Space,
  Checkbox,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../../helpers';

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

  const requestEndpointOptions = [
    { value: 'openai', label: 'OpenAI' },
    { value: 'gemini', label: 'Gemini' },
    { value: 'openai_mod', label: 'OpenAI魔改' },
  ];

  const resolutionOptions = [
    { value: '1K', label: '1K' },
    { value: '2K', label: '2K' },
    { value: '4K', label: '4K' },
  ];

  const aspectRatioOptions = [
    { value: 'auto', label: 'Auto' },
    { value: '1:1', label: '1:1' },
    { value: '2:3', label: '2:3' },
    { value: '3:2', label: '3:2' },
    { value: '3:4', label: '3:4' },
    { value: '4:3', label: '4:3' },
    { value: '4:5', label: '4:5' },
    { value: '5:4', label: '5:4' },
    { value: '9:16', label: '9:16' },
    { value: '16:9', label: '16:9' },
    { value: '21:9', label: '21:9' },
  ];

  useEffect(() => {
    if (visible && formApi) {
      if (editingMapping) {
        // 解析 JSON 字符串为数组
        let resolutions = [];
        let aspectRatios = [];
        try {
          if (editingMapping.resolutions) {
            resolutions = JSON.parse(editingMapping.resolutions);
          }
          if (editingMapping.aspect_ratios) {
            aspectRatios = JSON.parse(editingMapping.aspect_ratios);
          }
        } catch (e) {
          console.error('Failed to parse resolutions or aspect_ratios:', e);
        }

        setSelectedResolutions(resolutions);
        setSelectedAspectRatios(aspectRatios);

        formApi.setValues({
          ...editingMapping,
          resolutions,
          aspect_ratios: aspectRatios,
        });
      } else {
        setSelectedResolutions([]);
        setSelectedAspectRatios([]);

        formApi.setValues({
          request_model: '',
          display_name: '',
          model_series: '',
          model_type: 1,
          description: '',
          request_endpoint: '',
          resolutions: [],
          aspect_ratios: [],
        });
      }
    }
  }, [visible, editingMapping, formApi]);

  const handleSubmit = async (values) => {
    setLoading(true);
    try {
      const payload = {
        ...values,
        // 将数组转换为 JSON 字符串
        resolutions: values.resolutions ? JSON.stringify(values.resolutions) : '',
        aspect_ratios: values.aspect_ratios ? JSON.stringify(values.aspect_ratios) : '',
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
    const allValues = aspectRatioOptions.map(opt => opt.value);
    setSelectedAspectRatios(allValues);
    formApi?.setValue('aspect_ratios', allValues);
  };

  const handleDeselectAllAspectRatios = () => {
    setSelectedAspectRatios([]);
    formApi?.setValue('aspect_ratios', []);
  };

  const handleSelectAllResolutions = () => {
    const allValues = resolutionOptions.map(opt => opt.value);
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
        />
        <Form.Select
          field='request_endpoint'
          label={t('请求端点')}
          placeholder={t('选择请求端点类型')}
          optionList={requestEndpointOptions}
          rules={[{ required: true, message: t('请选择请求端点') }]}
        />
        <div>
          <div style={{ marginBottom: 8, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
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
          <div style={{ marginBottom: 8, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
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
        <Space style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-end' }}>
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
