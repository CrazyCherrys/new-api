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
import { useTranslation } from 'react-i18next';
import {
  Select,
  Button,
  Upload,
  Spin,
  Typography,
  Image,
  InputNumber,
  TextArea,
} from '@douyinfe/semi-ui';
import {
  IconPlus,
  IconDelete,
  IconClock,
  IconImage,
  IconBolt,
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../helpers';

const { Text } = Typography;

const ImageGeneration = () => {
  const { t } = useTranslation();

  const [loading, setLoading] = useState(false);
  const [modelSeries, setModelSeries] = useState([]);
  const [models, setModels] = useState([]);
  const [filteredModels, setFilteredModels] = useState([]);

  const [selectedSeries, setSelectedSeries] = useState('all');
  const [selectedModel, setSelectedModel] = useState('');
  const [prompt, setPrompt] = useState('');
  const [referenceImages, setReferenceImages] = useState([]);
  const [aspectRatio, setAspectRatio] = useState('');
  const [resolution, setResolution] = useState('');
  const [quantity, setQuantity] = useState(1);
  const [generatedImages, setGeneratedImages] = useState([]);
  const [generating, setGenerating] = useState(false);

  const [availableAspectRatios, setAvailableAspectRatios] = useState([]);
  const [availableResolutions, setAvailableResolutions] = useState([]);

  const [activeTab, setActiveTab] = useState('history');
  const [filterStatus, setFilterStatus] = useState('all');
  const [filterModel, setFilterModel] = useState('all');
  const [filterTime, setFilterTime] = useState('all');

  const formatModelSeries = (series) => {
    if (!series) return '';

    const seriesMap = {
      'openai': 'OpenAI',
      'gemini': 'Gemini',
      'claude': 'Claude',
      'grok': 'Grok',
      'deepseek': 'DeepSeek',
      'qwen': 'Qwen',
      'glm': 'GLM',
      'hunyuan': 'Hunyuan',
      'doubao': 'Doubao',
      'spark': 'Spark',
      'baichuan': 'Baichuan',
      'minimax': 'Minimax',
      'moonshot': 'Moonshot',
      'yi': 'Yi',
      'chatglm': 'ChatGLM',
      'ernie': 'ERNIE',
      'wenxin': 'Wenxin',
      'tongyi': 'Tongyi',
      'azure': 'Azure',
      'aws': 'AWS',
      'cohere': 'Cohere',
      'anthropic': 'Anthropic',
      'mistral': 'Mistral',
      'llama': 'Llama',
      'palm': 'PaLM',
      'bard': 'Bard',
      'midjourney': 'Midjourney',
      'dalle': 'DALL-E',
      'stable-diffusion': 'Stable Diffusion',
      'flux': 'Flux',
      'suno': 'Suno',
    };

    return seriesMap[series.toLowerCase()] || series.charAt(0).toUpperCase() + series.slice(1);
  };

  useEffect(() => {
    loadDrawingModels();
  }, []);

  const loadDrawingModels = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/model-mapping/search?model_type=2');
      if (res.data.success) {
        const drawingModels = res.data.data.items || [];
        setModels(drawingModels);

        const seriesSet = new Set();
        drawingModels.forEach((model) => {
          if (model.model_series) {
            seriesSet.add(model.model_series);
          }
        });
        setModelSeries(Array.from(seriesSet));
      } else {
        showError(res.data.message || t('加载模型失败'));
      }
    } catch (error) {
      showError(error.message || t('加载模型失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (selectedSeries === 'all') {
      const filtered = models.filter((model) => model.status === 1);
      setFilteredModels(filtered);
    } else if (selectedSeries) {
      const filtered = models.filter(
        (model) => model.model_series === selectedSeries && model.status === 1,
      );
      setFilteredModels(filtered);
    } else {
      setFilteredModels([]);
    }
    setSelectedModel('');
    setAvailableAspectRatios([]);
    setAvailableResolutions([]);
  }, [selectedSeries, models]);

  useEffect(() => {
    if (selectedModel) {
      const model = models.find((m) => m.request_model === selectedModel);
      if (model) {
        if (model.aspect_ratios) {
          try {
            const ratios = JSON.parse(model.aspect_ratios);
            setAvailableAspectRatios(ratios);
            if (ratios.length > 0) {
              setAspectRatio(ratios[0]);
            }
          } catch (e) {
            setAvailableAspectRatios([]);
          }
        }
        if (model.resolutions) {
          try {
            const resolutions = JSON.parse(model.resolutions);
            setAvailableResolutions(resolutions);
            if (resolutions.length > 0) {
              setResolution(resolutions[0]);
            }
          } catch (e) {
            setAvailableResolutions([]);
          }
        }
      }
    } else {
      setAvailableAspectRatios([]);
      setAvailableResolutions([]);
      setAspectRatio('');
      setResolution('');
    }
  }, [selectedModel, models]);

  const handleImageUpload = ({ fileList }) => {
    setReferenceImages(fileList);
  };

  const handleImageRemove = (file) => {
    setReferenceImages(referenceImages.filter((img) => img.uid !== file.uid));
  };

  const handleGenerate = async () => {
    if (!selectedModel) {
      showError(t('请选择模型'));
      return;
    }
    if (!prompt.trim()) {
      showError(t('请输入提示词'));
      return;
    }

    setGenerating(true);
    try {
      const params = {
        model: selectedModel,
        prompt: prompt,
        n: quantity,
      };

      if (aspectRatio) {
        params.aspect_ratio = aspectRatio;
      }
      if (resolution) {
        params.size = resolution;
      }

      if (referenceImages.length > 0) {
        const imagePromises = referenceImages.map((file) => {
          return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = (e) => resolve(e.target.result);
            reader.onerror = reject;
            reader.readAsDataURL(file.fileInstance);
          });
        });
        const base64Images = await Promise.all(imagePromises);
        params.reference_images = base64Images;
      }

      const res = await API.post('/v1/images/generations', params);
      if (res.data.data && res.data.data.length > 0) {
        setGeneratedImages(res.data.data);
        showSuccess(t('图片生成成功'));
      } else {
        showError(t('图片生成失败'));
      }
    } catch (error) {
      showError(error.message || t('图片生成失败'));
    } finally {
      setGenerating(false);
    }
  };

  const styles = {
    container: {
      display: 'flex',
      height: 'calc(100vh - 60px)',
      marginTop: 60,
      overflow: 'hidden',
    },
    leftPanel: {
      width: 320,
      minWidth: 320,
      display: 'flex',
      flexDirection: 'column',
      borderRight: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
    },
    leftContent: {
      flex: 1,
      overflowY: 'auto',
      padding: '20px 16px 0',
    },
    leftBottom: {
      padding: '16px',
      borderTop: '1px solid var(--semi-color-border)',
    },
    rightPanel: {
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      background: 'var(--semi-color-bg-1)',
      overflow: 'hidden',
    },
    rightTopBar: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '12px 16px',
      borderBottom: '1px solid var(--semi-color-border)',
      flexWrap: 'wrap',
      gap: 8,
    },
    rightContent: {
      flex: 1,
      overflow: 'auto',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    label: {
      display: 'block',
      fontSize: 13,
      fontWeight: 500,
      color: 'var(--semi-color-text-0)',
      marginBottom: 6,
    },
    fieldGroup: {
      marginBottom: 16,
    },
    tabBtn: (active) => ({
      display: 'inline-flex',
      alignItems: 'center',
      gap: 6,
      padding: '6px 14px',
      borderRadius: 20,
      border: 'none',
      cursor: 'pointer',
      fontSize: 13,
      fontWeight: 500,
      transition: 'all 0.2s',
      background: active
        ? 'var(--semi-color-primary-light-default)'
        : 'transparent',
      color: active
        ? 'var(--semi-color-primary)'
        : 'var(--semi-color-text-2)',
    }),
    tabDot: {
      width: 8,
      height: 8,
      borderRadius: '50%',
      background: '#ff6b35',
      display: 'inline-block',
    },
    generateBtn: {
      width: '100%',
      height: 44,
      borderRadius: 10,
      border: 'none',
      cursor: 'pointer',
      fontSize: 15,
      fontWeight: 600,
      color: '#fff',
      background: 'linear-gradient(135deg, #e8593c 0%, #d4a843 50%, #5a9e6f 100%)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 6,
      transition: 'opacity 0.2s',
      marginTop: 12,
    },
    addImageBtn: {
      width: 48,
      height: 48,
      borderRadius: 8,
      border: '1px dashed var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      cursor: 'pointer',
      color: 'var(--semi-color-text-2)',
      transition: 'border-color 0.2s',
    },
    textareaWrapper: {
      position: 'relative',
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      padding: 0,
    },
    charCount: {
      fontSize: 12,
      color: 'var(--semi-color-text-3)',
      padding: '4px 12px 8px',
      textAlign: 'left',
    },
    emptyState: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: 12,
    },
    emptyIcon: {
      width: 64,
      height: 64,
      borderRadius: '50%',
      background: 'rgba(232, 89, 60, 0.12)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    paramRow: {
      display: 'flex',
      gap: 12,
      alignItems: 'flex-end',
    },
    paramItem: {
      flex: 1,
    },
    paramLabel: {
      fontSize: 12,
      color: 'var(--semi-color-text-2)',
      marginBottom: 4,
      display: 'block',
    },
    referenceImageThumb: {
      width: 48,
      height: 48,
      borderRadius: 8,
      objectFit: 'cover',
      border: '1px solid var(--semi-color-border)',
    },
    referenceImageContainer: {
      position: 'relative',
      display: 'inline-block',
    },
    removeImageBtn: {
      position: 'absolute',
      top: -6,
      right: -6,
      width: 18,
      height: 18,
      borderRadius: '50%',
      background: 'var(--semi-color-danger)',
      border: 'none',
      cursor: 'pointer',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: '#fff',
      fontSize: 10,
      padding: 0,
    },
    filterGroup: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexWrap: 'wrap',
    },
    filterLabel: {
      fontSize: 13,
      color: 'var(--semi-color-text-2)',
    },
    imagesGrid: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
      gap: 16,
      padding: 16,
      width: '100%',
      alignContent: 'start',
    },
  };

  const renderLeftPanel = () => (
    <div style={styles.leftPanel}>
      <div style={styles.leftContent}>
        <Spin spinning={loading}>
          <div style={styles.fieldGroup}>
            <span style={styles.label}>{t('模型系列')}</span>
            <Select
              style={{ width: '100%' }}
              value={selectedSeries}
              onChange={setSelectedSeries}
              disabled={modelSeries.length === 0}
            >
              <Select.Option value='all'>{t('全部系列')}</Select.Option>
              {modelSeries.map((series) => (
                <Select.Option key={series} value={series}>
                  {formatModelSeries(series)}
                </Select.Option>
              ))}
            </Select>
          </div>

          <div style={styles.fieldGroup}>
            <span style={styles.label}>{t('模型')}</span>
            <Select
              style={{ width: '100%' }}
              value={selectedModel}
              onChange={setSelectedModel}
              disabled={filteredModels.length === 0}
              filter
              placeholder={t('请选择模型')}
            >
              {filteredModels.map((model) => (
                <Select.Option
                  key={model.request_model}
                  value={model.request_model}
                >
                  {model.display_name || model.request_model}
                </Select.Option>
              ))}
            </Select>
          </div>

          <div style={styles.fieldGroup}>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: 6,
              }}
            >
              <span style={styles.label}>{t('描述你的创意')}</span>
              <span
                style={{
                  fontSize: 12,
                  color: 'var(--semi-color-primary)',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 4,
                }}
              >
                <IconBolt size='small' />
                {t('AI优化')}
              </span>
            </div>
            <div style={styles.textareaWrapper}>
              <TextArea
                placeholder={t('描述你的创意...')}
                value={prompt}
                onChange={setPrompt}
                maxCount={5000}
                showClear
                autosize={{ minRows: 6, maxRows: 12 }}
                style={{
                  border: 'none',
                  background: 'transparent',
                  resize: 'none',
                }}
              />
              <div style={styles.charCount}>
                {prompt.length}/5000
              </div>
            </div>
          </div>

          <div style={styles.fieldGroup}>
            <span style={styles.label}>{t('参考图像')}</span>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
              {referenceImages.map((file, idx) => (
                <div key={file.uid || idx} style={styles.referenceImageContainer}>
                  <img
                    src={
                      file.url ||
                      (file.fileInstance &&
                        URL.createObjectURL(file.fileInstance))
                    }
                    alt=''
                    style={styles.referenceImageThumb}
                  />
                  <button
                    style={styles.removeImageBtn}
                    onClick={() => handleImageRemove(file)}
                  >
                    <IconDelete size='extra-small' />
                  </button>
                </div>
              ))}
              <Upload
                action=''
                accept='image/*'
                multiple
                fileList={referenceImages}
                onChange={handleImageUpload}
                showUploadList={false}
                beforeUpload={() => false}
              >
                <div style={styles.addImageBtn}>
                  <IconPlus size='large' />
                </div>
              </Upload>
            </div>
          </div>
        </Spin>
      </div>

      <div style={styles.leftBottom}>
        <div style={styles.paramRow}>
          <div style={styles.paramItem}>
            <span style={styles.paramLabel}>{t('生成比例')}</span>
            <Select
              style={{ width: '100%' }}
              value={aspectRatio || 'auto'}
              onChange={(val) => setAspectRatio(val === 'auto' ? '' : val)}
              size='default'
            >
              <Select.Option value='auto'>Auto</Select.Option>
              {availableAspectRatios.map((ratio) => (
                <Select.Option key={ratio} value={ratio}>
                  {ratio}
                </Select.Option>
              ))}
            </Select>
          </div>
          <div style={styles.paramItem}>
            <span style={styles.paramLabel}>{t('分辨率')}</span>
            <Select
              style={{ width: '100%' }}
              value={resolution || 'auto'}
              onChange={(val) => setResolution(val === 'auto' ? '' : val)}
              size='default'
            >
              <Select.Option value='auto'>Auto</Select.Option>
              {availableResolutions.map((res) => (
                <Select.Option key={res} value={res}>
                  {res}
                </Select.Option>
              ))}
            </Select>
          </div>
          <div style={styles.paramItem}>
            <span style={styles.paramLabel}>{t('生成数目')}</span>
            <InputNumber
              min={1}
              max={4}
              value={quantity}
              onChange={(val) => setQuantity(val || 1)}
              style={{ width: '100%' }}
            />
          </div>
        </div>

        <button
          style={{
            ...styles.generateBtn,
            opacity: generating || !selectedModel || !prompt.trim() ? 0.6 : 1,
            pointerEvents:
              generating || !selectedModel || !prompt.trim()
                ? 'none'
                : 'auto',
          }}
          onClick={handleGenerate}
          disabled={generating || !selectedModel || !prompt.trim()}
        >
          {generating ? (
            <Spin size='small' />
          ) : (
            <>
              <IconImage size='small' />
              {t('生成')}
            </>
          )}
        </button>
      </div>
    </div>
  );

  const renderRightPanel = () => (
    <div style={styles.rightPanel}>
      <div style={styles.rightTopBar}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <button
            style={styles.tabBtn(activeTab === 'history')}
            onClick={() => setActiveTab('history')}
          >
            <span style={styles.tabDot} />
            {t('生成记录')}
          </button>
          <button
            style={styles.tabBtn(activeTab === 'creative')}
            onClick={() => setActiveTab('creative')}
          >
            <IconImage size='small' />
            {t('创意')}
          </button>
        </div>

        <div style={styles.filterGroup}>
          <span style={styles.filterLabel}>{t('状态')}</span>
          <Select
            size='small'
            value={filterStatus}
            onChange={setFilterStatus}
            style={{ width: 80 }}
          >
            <Select.Option value='all'>{t('全部')}</Select.Option>
            <Select.Option value='success'>{t('成功')}</Select.Option>
            <Select.Option value='failed'>{t('失败')}</Select.Option>
          </Select>

          <span style={styles.filterLabel}>{t('模型')}</span>
          <Select
            size='small'
            value={filterModel}
            onChange={setFilterModel}
            style={{ width: 80 }}
          >
            <Select.Option value='all'>{t('全部')}</Select.Option>
          </Select>

          <span style={styles.filterLabel}>{t('时间')}</span>
          <Select
            size='small'
            value={filterTime}
            onChange={setFilterTime}
            style={{ width: 80 }}
          >
            <Select.Option value='all'>{t('全部')}</Select.Option>
          </Select>

          <Button
            size='small'
            type='danger'
            theme='solid'
            style={{ fontSize: 12 }}
          >
            {t('批量勾选')}
          </Button>
          <Button
            size='small'
            type='warning'
            theme='borderless'
            style={{ fontSize: 12 }}
          >
            {t('批量多选')}
          </Button>
          <Button size='small' theme='borderless' style={{ fontSize: 12 }}>
            {t('一键清除')}
          </Button>
          <Button
            size='small'
            type='danger'
            theme='borderless'
            style={{ fontSize: 12 }}
          >
            {t('删除所选')}
          </Button>
          <Text type='tertiary' size='small'>
            {t('已选 0 张')}
          </Text>
        </div>
      </div>

      <div
        style={{
          ...styles.rightContent,
          ...(generatedImages.length > 0
            ? { alignItems: 'flex-start', justifyContent: 'flex-start' }
            : {}),
        }}
      >
        {generatedImages.length > 0 ? (
          <div style={styles.imagesGrid}>
            {generatedImages.map((img, index) => (
              <div
                key={index}
                style={{
                  borderRadius: 8,
                  overflow: 'hidden',
                  border: '1px solid var(--semi-color-border)',
                }}
              >
                <Image
                  src={img.url}
                  alt={`Generated ${index + 1}`}
                  width='100%'
                  preview
                  style={{ display: 'block' }}
                />
              </div>
            ))}
          </div>
        ) : (
          <div style={styles.emptyState}>
            <div style={styles.emptyIcon}>
              <IconClock
                size='extra-large'
                style={{ color: '#e8593c', fontSize: 28 }}
              />
            </div>
            <Text
              strong
              style={{ fontSize: 16, color: 'var(--semi-color-text-0)' }}
            >
              {t('暂无生成录')}
            </Text>
            <Text
              type='tertiary'
              style={{ fontSize: 13, textAlign: 'center', maxWidth: 280 }}
            >
              {t('完成一次生成后，这里会保留你的创作历史记录。')}
            </Text>
          </div>
        )}
      </div>
    </div>
  );

  return (
    <div style={styles.container}>
      {renderLeftPanel()}
      {renderRightPanel()}
    </div>
  );
};

export default ImageGeneration;
