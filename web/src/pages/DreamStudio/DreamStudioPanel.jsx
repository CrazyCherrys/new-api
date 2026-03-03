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
import { Card, Form, Input, Select, Button, Upload, InputNumber, Layout, Spin, Progress } from '@douyinfe/semi-ui';
import { IconChevronLeft, IconChevronRight, IconUpload } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { getImageConfig } from '../../helpers/imageApi';
import { showError, showSuccess } from '../../helpers';
import { API } from '../../helpers/api';

const { Sider, Content } = Layout;
const { TextArea } = Input;

const DreamStudioPanel = ({
  prompt,
  setPrompt,
  model,
  setModel,
  resolution,
  setResolution,
  aspectRatio,
  setAspectRatio,
  referenceImage,
  setReferenceImage,
  count,
  setCount,
  refreshHistory,
}) => {
  const { t } = useTranslation();
  const [collapsed, setCollapsed] = useState(false);
  const [loading, setLoading] = useState(false);
  const [visibleModels, setVisibleModels] = useState([]);
  const [uploading, setUploading] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [progress, setProgress] = useState({ current: 0, total: 0, message: '' });

  // 分辨率选项
  const resolutionOptions = [
    { label: '1024x1024', value: '1024x1024' },
    { label: '1152x896', value: '1152x896' },
    { label: '896x1152', value: '896x1152' },
    { label: '1216x832', value: '1216x832' },
    { label: '832x1216', value: '832x1216' },
    { label: '1344x768', value: '1344x768' },
    { label: '768x1344', value: '768x1344' },
    { label: '1536x640', value: '1536x640' },
    { label: '640x1536', value: '640x1536' },
  ];

  // 宽高比选项
  const aspectRatioOptions = [
    { label: '1:1', value: '1:1' },
    { label: '16:9', value: '16:9' },
    { label: '9:16', value: '9:16' },
    { label: '4:3', value: '4:3' },
    { label: '3:4', value: '3:4' },
    { label: '21:9', value: '21:9' },
    { label: '9:21', value: '9:21' },
  ];

  // 加载配置
  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    setLoading(true);
    try {
      const res = await getImageConfig();
      if (res.data?.success && res.data?.data) {
        const config = res.data.data;
        if (config.visible_models && config.visible_models.length > 0) {
          setVisibleModels(config.visible_models);
          if (!model && config.default_model) {
            setModel(config.default_model);
          }
        }
        if (config.default_resolution && !resolution) {
          setResolution(config.default_resolution);
        }
        if (config.default_aspect_ratio && !aspectRatio) {
          setAspectRatio(config.default_aspect_ratio);
        }
      }
    } catch (error) {
      showError(t('加载配置失败'));
    } finally {
      setLoading(false);
    }
  };

  // 上传参考图
  const handleUpload = async ({ file, onSuccess, onError }) => {
    setUploading(true);
    const formData = new FormData();
    formData.append('file', file.fileInstance);

    try {
      const res = await API.post('/api/v1/image-tasks/upload-reference', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
      });

      if (res.data?.success && res.data?.data) {
        setReferenceImage(res.data.data);
        onSuccess();
      } else {
        showError(res.data?.message || t('上传失败'));
        onError();
      }
    } catch (error) {
      showError(t('上传失败，请重试'));
      onError();
    } finally {
      setUploading(false);
    }
  };

  // 移除参考图
  const handleRemove = () => {
    setReferenceImage(null);
  };

  // 生成图像
  const handleGenerate = async () => {
    // 验证必填字段
    if (!prompt.trim()) {
      showError(t('请输入提示词'));
      return;
    }
    if (!model) {
      showError(t('请选择模型'));
      return;
    }

    setGenerating(true);
    setProgress({ current: 0, total: count, message: '' });
    const tasks = [];
    let successCount = 0;

    for (let i = 0; i < count; i++) {
      setProgress({
        current: i,
        total: count,
        message: t('正在创建') + ` ${i + 1}/${count} ` + t('个任务...')
      });

      try {
        const payload = {
          model_id: model,
          prompt,
          resolution,
          aspect_ratio: aspectRatio,
          count: 1
        };

        // 如果有参考图，添加到请求中
        if (referenceImage) {
          payload.reference_image = referenceImage;
        }

        const response = await API.post('/api/v1/image-tasks/generate', payload);

        if (response.data?.success && response.data?.data?.task_id) {
          tasks.push(response.data.data.task_id);
          successCount++;
        } else {
          showError(t('任务') + ` ${i + 1} ` + t('创建失败') + `: ${response.data?.message || t('未知错误')}`);
        }

        // 避免速率限制，任务间延迟
        if (i < count - 1) {
          await new Promise(resolve => setTimeout(resolve, 500));
        }
      } catch (error) {
        showError(t('任务') + ` ${i + 1} ` + t('创建失败') + `: ${error.message || t('网络错误')}`);
      }
    }

    setProgress({ current: count, total: count, message: '' });
    setGenerating(false);

    if (successCount > 0) {
      showSuccess(t('成功创建') + ` ${successCount} ` + t('个任务'));
      if (refreshHistory) {
        refreshHistory();
      }
    }
  };

  const siderStyle = {
    width: collapsed ? 0 : 320,
    minWidth: collapsed ? 0 : 320,
    maxWidth: collapsed ? 0 : 320,
    transition: 'all 0.2s',
    overflow: 'hidden',
    backgroundColor: '#ffffff',
    borderRight: '1px solid #e8e8e8',
  };

  const contentStyle = {
    backgroundColor: '#f7f8fa',
    minHeight: 'calc(100vh - 60px)',
  };

  const toggleButtonStyle = {
    position: 'absolute',
    left: '8px',
    top: '8px',
    zIndex: 10,
  };

  const mainContentStyle = {
    padding: '16px',
    paddingTop: '48px',
    maxWidth: '1200px',
    margin: '0 auto',
  };

  const cardStyle = {
    marginBottom: '16px',
  };

  const formItemStyle = {
    marginBottom: '16px',
  };

  return (
    <Layout style={{ minHeight: 'calc(100vh - 60px)' }}>
      <Sider style={siderStyle}>
        <div style={{ padding: '16px', height: '100%', overflowY: 'auto' }}>
          <h3 style={{ fontSize: '18px', fontWeight: 600, marginBottom: '16px' }}>
            {t('生成参数')}
          </h3>

          <Spin spinning={loading}>
            <Form>
              <div style={formItemStyle}>
                <Form.Label>{t('模型')}</Form.Label>
                <Select
                  value={model}
                  onChange={setModel}
                  placeholder={t('选择模型')}
                  style={{ width: '100%' }}
                  disabled={visibleModels.length === 0}
                >
                  {visibleModels.map((m) => (
                    <Select.Option key={m} value={m}>
                      {m}
                    </Select.Option>
                  ))}
                </Select>
              </div>

              <div style={formItemStyle}>
                <Form.Label>{t('分辨率')}</Form.Label>
                <Select
                  value={resolution}
                  onChange={setResolution}
                  placeholder={t('选择分辨率')}
                  style={{ width: '100%' }}
                >
                  {resolutionOptions.map((opt) => (
                    <Select.Option key={opt.value} value={opt.value}>
                      {opt.label}
                    </Select.Option>
                  ))}
                </Select>
              </div>

              <div style={formItemStyle}>
                <Form.Label>{t('宽高比')}</Form.Label>
                <Select
                  value={aspectRatio}
                  onChange={setAspectRatio}
                  placeholder={t('选择宽高比')}
                  style={{ width: '100%' }}
                >
                  {aspectRatioOptions.map((opt) => (
                    <Select.Option key={opt.value} value={opt.value}>
                      {opt.label}
                    </Select.Option>
                  ))}
                </Select>
              </div>

              <div style={formItemStyle}>
                <Form.Label>{t('生成数量')}</Form.Label>
                <InputNumber
                  value={count}
                  onChange={setCount}
                  min={1}
                  max={10}
                  style={{ width: '100%' }}
                />
              </div>
            </Form>
          </Spin>
        </div>
      </Sider>

      <Layout>
        <Content style={contentStyle}>
          <div style={{ position: 'relative', height: '100%' }}>
            <Button
              icon={collapsed ? <IconChevronRight /> : <IconChevronLeft />}
              onClick={() => setCollapsed(!collapsed)}
              style={toggleButtonStyle}
              size="small"
            />

            <div style={mainContentStyle}>
              <Card style={cardStyle}>
                <h3 style={{ fontSize: '16px', fontWeight: 600, marginBottom: '12px' }}>
                  {t('提示词')}
                </h3>
                <TextArea
                  value={prompt}
                  onChange={setPrompt}
                  placeholder={t('描述你想要生成的图像...')}
                  autosize={{ minRows: 4, maxRows: 8 }}
                  style={{ width: '100%' }}
                  disabled={generating}
                />

                <div style={{ marginTop: '16px' }}>
                  <Button
                    type="primary"
                    size="large"
                    onClick={handleGenerate}
                    disabled={generating || !prompt.trim() || !model}
                    loading={generating}
                    style={{ width: '100%' }}
                  >
                    {generating ? t('生成中...') : t('开始生成')}
                  </Button>
                </div>

                {generating && progress.total > 1 && (
                  <div style={{ marginTop: '16px' }}>
                    <Progress
                      percent={(progress.current / progress.total) * 100}
                      showInfo={true}
                      format={() => `${progress.current}/${progress.total}`}
                    />
                    {progress.message && (
                      <div style={{ marginTop: '8px', fontSize: '12px', color: '#8c8c8c', textAlign: 'center' }}>
                        {progress.message}
                      </div>
                    )}
                  </div>
                )}
              </Card>

              <Card style={cardStyle}>
                <h3 style={{ fontSize: '16px', fontWeight: 600, marginBottom: '12px' }}>
                  {t('参考图片')}
                </h3>
                <Upload
                  action=""
                  customRequest={handleUpload}
                  onRemove={handleRemove}
                  accept="image/*"
                  limit={1}
                  disabled={uploading}
                  fileList={referenceImage ? [{ uid: '1', name: referenceImage.filename || 'reference.png', status: 'success' }] : []}
                >
                  <Button icon={<IconUpload />} disabled={uploading} loading={uploading}>
                    {t('上传参考图')}
                  </Button>
                </Upload>
                <div style={{ marginTop: '8px', fontSize: '12px', color: '#8c8c8c' }}>
                  {t('支持 JPG、PNG 格式，最大 10MB')}
                </div>
              </Card>

              <Card style={cardStyle}>
                <h3 style={{ fontSize: '16px', fontWeight: 600, marginBottom: '12px' }}>
                  {t('历史任务')}
                </h3>
                <div style={{ fontSize: '14px', color: '#8c8c8c' }}>
                  {t('历史任务画廊将在后续步骤实现')}
                </div>
              </Card>
            </div>
          </div>
        </Content>
      </Layout>
    </Layout>
  );
};

export default DreamStudioPanel;
