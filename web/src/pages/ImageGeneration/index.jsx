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
  Card,
  Select,
  Input,
  Button,
  Upload,
  Space,
  Spin,
  Typography,
  Toast,
  Image,
  Row,
  Col,
} from '@douyinfe/semi-ui';
import { IconUpload, IconDelete } from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../helpers';

const { Title, Text } = Typography;

const ImageGeneration = () => {
  const { t } = useTranslation();

  // 状态管理
  const [loading, setLoading] = useState(false);
  const [modelSeries, setModelSeries] = useState([]);
  const [models, setModels] = useState([]);
  const [filteredModels, setFilteredModels] = useState([]);

  const [selectedSeries, setSelectedSeries] = useState('');
  const [selectedModel, setSelectedModel] = useState('');
  const [prompt, setPrompt] = useState('');
  const [referenceImages, setReferenceImages] = useState([]);
  const [aspectRatio, setAspectRatio] = useState('');
  const [resolution, setResolution] = useState('');
  const [quantity, setQuantity] = useState(1);
  const [generatedImages, setGeneratedImages] = useState([]);
  const [generating, setGenerating] = useState(false);

  // 可用的宽高比和分辨率选项
  const [availableAspectRatios, setAvailableAspectRatios] = useState([]);
  const [availableResolutions, setAvailableResolutions] = useState([]);

  // 加载绘画模型
  useEffect(() => {
    loadDrawingModels();
  }, []);

  const loadDrawingModels = async () => {
    setLoading(true);
    try {
      // 获取模型类型为2（绘画）的模型
      const res = await API.get('/api/model-mapping/search?model_type=2');
      if (res.data.success) {
        const drawingModels = res.data.data.items || [];
        setModels(drawingModels);

        // 提取唯一的模型系列
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

  // 当选择模型系列时，过滤对应的模型
  useEffect(() => {
    if (selectedSeries) {
      const filtered = models.filter(
        (model) => model.model_series === selectedSeries && model.status === 1
      );
      setFilteredModels(filtered);
      setSelectedModel('');
      setAvailableAspectRatios([]);
      setAvailableResolutions([]);
    } else {
      setFilteredModels([]);
      setSelectedModel('');
    }
  }, [selectedSeries, models]);

  // 当选择具体模型时，加载该模型的宽高比和分辨率
  useEffect(() => {
    if (selectedModel) {
      const model = models.find((m) => m.request_model === selectedModel);
      if (model) {
        // 解析宽高比
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

        // 解析分辨率
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

  // 处理图片上传
  const handleImageUpload = ({ fileList }) => {
    setReferenceImages(fileList);
  };

  // 处理图片删除
  const handleImageRemove = (file) => {
    setReferenceImages(referenceImages.filter((img) => img.uid !== file.uid));
  };

  // 生成图片
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
      // 构建请求参数
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

      // 如果有参考图，转换为base64
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

      // 调用API生成图片
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

  return (
    <div className='mt-[60px] px-2'>
      <Card bordered>
        <Spin spinning={loading}>
          <Space vertical spacing='large' style={{ width: '100%' }}>
            {/* 标题 */}
            <Title heading={3}>{t('AI绘画')}</Title>
            {/* 模型系列选择 */}
            <div>
              <Text strong>{t('模型系列')}</Text>
              <Select
                placeholder={t('请选择模型系列')}
                style={{ width: '100%', marginTop: 8 }}
                value={selectedSeries}
                onChange={setSelectedSeries}
                disabled={modelSeries.length === 0}
              >
                {modelSeries.map((series) => (
                  <Select.Option key={series} value={series}>
                    {series}
                  </Select.Option>
                ))}
              </Select>
            </div>

            {/* 模型选择 */}
            <div>
              <Text strong>{t('模型')}</Text>
              <Select
                placeholder={t('请选择模型')}
                style={{ width: '100%', marginTop: 8 }}
                value={selectedModel}
                onChange={setSelectedModel}
                disabled={!selectedSeries || filteredModels.length === 0}
                filter
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

            {/* 提示词输入 */}
            <div>
              <Text strong>{t('提示词')}</Text>
              <Input
                placeholder={t('请输入提示词描述您想要生成的图片')}
                style={{ marginTop: 8 }}
                value={prompt}
                onChange={setPrompt}
                maxLength={2000}
                showClear
              />
            </div>

            {/* 参考图上传 */}
            <div>
              <Text strong>{t('参考图')}</Text>
              <Text type='tertiary' size='small' style={{ marginLeft: 8 }}>
                ({t('可选')})
              </Text>
              <div style={{ marginTop: 8 }}>
                <Upload
                  action=''
                  accept='image/*'
                  multiple
                  fileList={referenceImages}
                  onChange={handleImageUpload}
                  onRemove={handleImageRemove}
                  listType='picture'
                  beforeUpload={() => false}
                >
                  <Button icon={<IconUpload />} theme='light'>
                    {t('上传参考图')}
                  </Button>
                </Upload>
              </div>
            </div>

            {/* 宽高比和分辨率 */}
            <Row gutter={16}>
              <Col span={12}>
                <div>
                  <Text strong>{t('宽高比')}</Text>
                  <Select
                    placeholder={t('请选择宽高比')}
                    style={{ width: '100%', marginTop: 8 }}
                    value={aspectRatio}
                    onChange={setAspectRatio}
                    disabled={availableAspectRatios.length === 0}
                  >
                    {availableAspectRatios.map((ratio) => (
                      <Select.Option key={ratio} value={ratio}>
                        {ratio}
                      </Select.Option>
                    ))}
                  </Select>
                </div>
              </Col>
              <Col span={12}>
                <div>
                  <Text strong>{t('分辨率')}</Text>
                  <Select
                    placeholder={t('请选择分辨率')}
                    style={{ width: '100%', marginTop: 8 }}
                    value={resolution}
                    onChange={setResolution}
                    disabled={availableResolutions.length === 0}
                  >
                    {availableResolutions.map((res) => (
                      <Select.Option key={res} value={res}>
                        {res}
                      </Select.Option>
                    ))}
                  </Select>
                </div>
              </Col>
            </Row>

            {/* 生成数量 */}
            <div>
              <Text strong>{t('生成数量')}</Text>
              <Select
                placeholder={t('请选择生成数量')}
                style={{ width: '100%', marginTop: 8 }}
                value={quantity}
                onChange={setQuantity}
              >
                {[1, 2, 3, 4].map((num) => (
                  <Select.Option key={num} value={num}>
                    {num}
                  </Select.Option>
                ))}
              </Select>
            </div>

            {/* 生成按钮 */}
            <Button
              theme='solid'
              type='primary'
              size='large'
              block
              onClick={handleGenerate}
              loading={generating}
              disabled={!selectedModel || !prompt.trim()}
            >
              {generating ? t('生成中...') : t('生成图片')}
            </Button>

            {/* 生成结果展示 */}
            {generatedImages.length > 0 && (
              <div style={{ marginTop: 24 }}>
                <Text strong style={{ fontSize: 16 }}>
                  {t('生成结果')}
                </Text>
                <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
                  {generatedImages.map((img, index) => (
                    <Col key={index} span={12}>
                      <Image
                        src={img.url}
                        alt={`Generated ${index + 1}`}
                        width='100%'
                        preview
                      />
                    </Col>
                  ))}
                </Row>
              </div>
            )}
          </Space>
        </Spin>
      </Card>
    </div>
  );
};

export default ImageGeneration;
