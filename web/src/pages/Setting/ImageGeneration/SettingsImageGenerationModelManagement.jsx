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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Col,
  Empty,
  Input,
  InputNumber,
  Popconfirm,
  Row,
  Select,
  Spin,
  Switch,
  Tag,
  TagInput,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { showError, showSuccess, showWarning } from '../../../helpers';
import { updateImageConfig } from '../../../helpers/imageApi';
import {
  aspectRatioOptions,
  createDefaultModelSetting,
  defaultImageGenerationInputs,
  deriveLegacyFieldsFromModelSettings,
  modelConfigKeys,
  pickFields,
  transformToBackend,
  imageResolutionOptions,
} from './shared';

const modelTypeOptions = [
  { value: 'image', label: 'Image' },
  { value: 'video', label: 'Video' },
  { value: 'text', label: 'Text' },
];

const requestEndpointOptions = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'openai_mod', label: 'OpenAI Mod' },
];

const modelFilterOptions = [
  { value: 'all', label: '全部' },
  { value: 'image', label: 'Image' },
  { value: 'video', label: 'Video' },
  { value: 'text', label: 'Text' },
];

export default function SettingsImageGenerationModelManagement({
  options,
  refresh,
}) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [newModelId, setNewModelId] = useState('');
  const [newModelName, setNewModelName] = useState('');
  const [newModelType, setNewModelType] = useState('image');
  const [searchQuery, setSearchQuery] = useState('');
  const [modelFilter, setModelFilter] = useState('all');

  const modelOptions = useMemo(
    () =>
      deriveLegacyFieldsFromModelSettings(
        pickFields(
          {
            ...defaultImageGenerationInputs,
            ...options,
          },
          modelConfigKeys,
        ),
      ),
    [options],
  );

  const [inputs, setInputs] = useState(modelOptions);
  const [inputsRow, setInputsRow] = useState(modelOptions);

  useEffect(() => {
    setInputs(modelOptions);
    setInputsRow(structuredClone(modelOptions));
  }, [modelOptions]);

  const modelEntries = useMemo(() => {
    return Object.entries(inputs.model_settings || {});
  }, [inputs.model_settings]);

  const filteredModelEntries = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    return modelEntries.filter(([modelId, modelSetting]) => {
      if (modelFilter !== 'all' && modelSetting.model_type !== modelFilter) {
        return false;
      }
      if (!query) {
        return true;
      }
      return (
        modelId.toLowerCase().includes(query) ||
        (modelSetting.display_name || '').toLowerCase().includes(query) ||
        (modelSetting.request_model_id || '').toLowerCase().includes(query)
      );
    });
  }, [modelEntries, modelFilter, searchQuery]);

  const applyModelSettings = (nextModelSettings, extraPatch = {}) => {
    const next = deriveLegacyFieldsFromModelSettings({
      ...inputs,
      ...extraPatch,
      model_settings: nextModelSettings,
    });
    setInputs(next);
  };

  const updateModelSetting = (modelId, patch) => {
    const currentSetting = inputs.model_settings[modelId] || {};
    const nextModelSettings = {
      ...inputs.model_settings,
      [modelId]: createDefaultModelSetting(
        modelId,
        { ...currentSetting, ...patch },
        inputs,
      ),
    };
    applyModelSettings(nextModelSettings);
  };

  const removeModel = (modelId) => {
    const nextModelSettings = { ...inputs.model_settings };
    delete nextModelSettings[modelId];
    applyModelSettings(nextModelSettings);
  };

  const addModel = () => {
    const modelId = newModelId.trim();
    if (!modelId) {
      showWarning(t('请填写模型 ID'));
      return;
    }
    if (inputs.model_settings[modelId]) {
      showWarning(t('模型已存在'));
      return;
    }

    const nextModelSettings = {
      ...inputs.model_settings,
      [modelId]: createDefaultModelSetting(
        modelId,
        {
          display_name: newModelName.trim() || modelId,
          request_model_id: modelId,
          model_type: newModelType,
        },
        inputs,
      ),
    };
    applyModelSettings(nextModelSettings, {
      default_model: inputs.default_model || modelId,
    });
    setNewModelId('');
    setNewModelName('');
    setNewModelType('image');
  };

  async function onSubmit() {
    if (JSON.stringify(inputs) === JSON.stringify(inputsRow)) {
      return showWarning(t('你似乎并没有修改什么'));
    }

    setLoading(true);
    try {
      const merged = {
        ...defaultImageGenerationInputs,
        ...options,
        ...inputs,
      };
      const res = await updateImageConfig(transformToBackend(merged));
      if (res.data?.success) {
        showSuccess(t('保存成功'));
        setInputsRow(structuredClone(inputs));
        await refresh?.();
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Spin spinning={loading}>
      <Typography.Title heading={6} style={{ marginTop: 0 }}>
        {t('模型管理')}
      </Typography.Title>

      <Row gutter={12}>
        <Col xs={24} sm={8}>
          <Input
            value={newModelId}
            placeholder={t('模型 ID，例如：dall-e-3')}
            onChange={setNewModelId}
          />
        </Col>
        <Col xs={24} sm={8}>
          <Input
            value={newModelName}
            placeholder={t('模型展示名称（可选）')}
            onChange={setNewModelName}
          />
        </Col>
        <Col xs={24} sm={5}>
          <Select
            value={newModelType}
            optionList={modelTypeOptions}
            onChange={setNewModelType}
          />
        </Col>
        <Col xs={24} sm={3}>
          <Button type='primary' style={{ width: '100%' }} onClick={addModel}>
            {t('添加模型')}
          </Button>
        </Col>
      </Row>

      <Row gutter={12} style={{ marginTop: 12, marginBottom: 12 }}>
        <Col xs={24} sm={8}>
          <Select
            value={inputs.default_model}
            optionList={modelEntries.map(([modelId, modelSetting]) => ({
              value: modelId,
              label: modelSetting.display_name || modelId,
            }))}
            placeholder={t('选择默认模型')}
            onChange={(value) =>
              setInputs((prev) => ({
                ...prev,
                default_model: value,
              }))
            }
            disabled={modelEntries.length === 0}
          />
        </Col>
        <Col xs={24} sm={8}>
          <Input
            value={searchQuery}
            placeholder={t('搜索模型 ID / 名称')}
            onChange={setSearchQuery}
          />
        </Col>
        <Col xs={24} sm={8}>
          <Select
            value={modelFilter}
            optionList={modelFilterOptions.map((item) => ({
              ...item,
              label: t(item.label),
            }))}
            onChange={setModelFilter}
          />
        </Col>
      </Row>

      <div style={{ marginBottom: 8 }}>
        <Tag color='light-blue'>
          {t('已配置模型')}：{modelEntries.length}
        </Tag>
      </div>

      {filteredModelEntries.length === 0 ? (
        <Empty description={t('暂无模型配置')} />
      ) : (
        filteredModelEntries.map(([modelId, modelSetting]) => (
          <Card key={modelId} style={{ marginTop: 12 }}>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: 12,
              }}
            >
              <div>
                <Typography.Text strong>
                  {modelSetting.display_name || modelId}
                </Typography.Text>
                <Typography.Text
                  type='tertiary'
                  style={{ marginLeft: 8, fontSize: 12 }}
                >
                  {modelId}
                </Typography.Text>
              </div>
              <Popconfirm
                title={t('确定删除该模型配置吗？')}
                content={modelId}
                onConfirm={() => removeModel(modelId)}
              >
                <Button type='danger' theme='borderless' size='small'>
                  {t('删除')}
                </Button>
              </Popconfirm>
            </div>

            <Row gutter={12}>
              <Col xs={24} sm={12}>
                <Input
                  value={modelSetting.display_name}
                  placeholder={t('展示名称')}
                  onChange={(value) =>
                    updateModelSetting(modelId, { display_name: value })
                  }
                />
              </Col>
              <Col xs={24} sm={12}>
                <Input
                  value={modelSetting.request_model_id}
                  placeholder={t('请求模型 ID')}
                  onChange={(value) =>
                    updateModelSetting(modelId, { request_model_id: value })
                  }
                />
              </Col>
            </Row>

            <Row gutter={12} style={{ marginTop: 12 }}>
              <Col xs={24} sm={6}>
                <Select
                  value={modelSetting.model_type}
                  optionList={modelTypeOptions}
                  onChange={(value) =>
                    updateModelSetting(modelId, { model_type: value })
                  }
                />
              </Col>
              <Col xs={24} sm={6}>
                <Select
                  value={modelSetting.request_endpoint}
                  optionList={requestEndpointOptions}
                  onChange={(value) =>
                    updateModelSetting(modelId, { request_endpoint: value })
                  }
                />
              </Col>
              <Col xs={24} sm={6}>
                <InputNumber
                  value={modelSetting.max_image_count}
                  min={1}
                  max={20}
                  style={{ width: '100%' }}
                  placeholder={t('最大生成数量')}
                  onChange={(value) =>
                    updateModelSetting(modelId, { max_image_count: value })
                  }
                />
              </Col>
              <Col xs={24} sm={6}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Switch
                    checked={modelSetting.rpm_enabled}
                    onChange={(value) =>
                      updateModelSetting(modelId, {
                        rpm_enabled: value,
                      })
                    }
                  />
                  <InputNumber
                    value={modelSetting.rpm_limit}
                    min={1}
                    max={2000}
                    disabled={!modelSetting.rpm_enabled}
                    placeholder='RPM'
                    style={{ flex: 1 }}
                    onChange={(value) =>
                      updateModelSetting(modelId, { rpm_limit: value })
                    }
                  />
                </div>
              </Col>
            </Row>

            <Row gutter={12} style={{ marginTop: 12 }}>
              <Col xs={24} sm={12}>
                <Select
                  value={modelSetting.default_resolution}
                  optionList={imageResolutionOptions.map((item) => ({
                    value: item,
                    label: item,
                  }))}
                  onChange={(value) =>
                    updateModelSetting(modelId, { default_resolution: value })
                  }
                />
              </Col>
              <Col xs={24} sm={12}>
                <Select
                  value={modelSetting.default_aspect_ratio}
                  optionList={aspectRatioOptions.map((item) => ({
                    value: item,
                    label: item,
                  }))}
                  onChange={(value) =>
                    updateModelSetting(modelId, { default_aspect_ratio: value })
                  }
                />
              </Col>
            </Row>

            <Row gutter={12} style={{ marginTop: 12 }}>
              <Col xs={24}>
                <TagInput
                  value={modelSetting.resolutions}
                  placeholder={t('可用分辨率（回车添加）')}
                  addOnBlur
                  onChange={(value) =>
                    updateModelSetting(modelId, { resolutions: value })
                  }
                />
              </Col>
            </Row>
            <Row gutter={12} style={{ marginTop: 12 }}>
              <Col xs={24}>
                <TagInput
                  value={modelSetting.aspect_ratios}
                  placeholder={t('可用宽高比（回车添加）')}
                  addOnBlur
                  onChange={(value) =>
                    updateModelSetting(modelId, { aspect_ratios: value })
                  }
                />
              </Col>
            </Row>
            {modelSetting.model_type === 'video' && (
              <Row gutter={12} style={{ marginTop: 12 }}>
                <Col xs={24}>
                  <TagInput
                    value={modelSetting.durations}
                    placeholder={t('可用时长（秒）')}
                    addOnBlur
                    onChange={(value) =>
                      updateModelSetting(modelId, { durations: value })
                    }
                  />
                </Col>
              </Row>
            )}
          </Card>
        ))
      )}

      <div style={{ marginTop: 16 }}>
        <Button type='primary' onClick={onSubmit}>
          {t('保存模型管理设置')}
        </Button>
      </div>
    </Spin>
  );
}
