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

import React, { useState, useEffect, useCallback } from 'react';
import {
  Card,
  Select,
  Space,
  Pagination,
  Spin,
  Button,
  Empty,
  DatePicker,
} from '@douyinfe/semi-ui';
import { IconRefresh, IconImage } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { showError } from '../../../helpers';
import { listImageTasks } from '../../../api/image';
import ImageGrid from '../components/ImageGrid';
import ImageDetailModal from '../components/ImageDetailModal';

const GalleryView = ({ onRegenerate }) => {
  const { t } = useTranslation();

  const [images, setImages] = useState([]);
  const [loading, setLoading] = useState(false);
  const [selectedImage, setSelectedImage] = useState(null);
  const [modalVisible, setModalVisible] = useState(false);
  const [filters, setFilters] = useState({
    model: 'all',
    dateRange: null,
  });
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 20,
    total: 0,
  });

  // 获取成功的图片任务
  const fetchImages = useCallback(async () => {
    setLoading(true);
    try {
      const params = {
        page: pagination.current,
        page_size: pagination.pageSize,
        status: 'succeeded', // 只获取成功的任务
      };

      if (filters.model !== 'all') {
        params.model = filters.model;
      }

      if (filters.dateRange && filters.dateRange.length === 2) {
        params.start_time = Math.floor(filters.dateRange[0].getTime() / 1000);
        params.end_time = Math.floor(filters.dateRange[1].getTime() / 1000);
      }

      const res = await listImageTasks(params);
      const { success, message, data } = res;

      if (success) {
        setImages(data.tasks || []);
        setPagination((prev) => ({
          ...prev,
          total: data.total || 0,
        }));
      } else {
        showError(message || t('获取图片列表失败'));
      }
    } catch (error) {
      console.error('Fetch images error:', error);
      showError(t('获取图片列表失败，请重试'));
    } finally {
      setLoading(false);
    }
  }, [pagination.current, pagination.pageSize, filters, t]);

  useEffect(() => {
    fetchImages();
  }, [fetchImages]);

  // 处理过滤器变化
  const handleFilterChange = (field, value) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value,
    }));
    setPagination((prev) => ({
      ...prev,
      current: 1,
    }));
  };

  // 处理分页变化
  const handlePageChange = (page) => {
    setPagination((prev) => ({
      ...prev,
      current: page,
    }));
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  // 处理页面大小变化
  const handlePageSizeChange = (size) => {
    setPagination((prev) => ({
      ...prev,
      pageSize: size,
      current: 1,
    }));
  };

  // 刷新列表
  const handleRefresh = () => {
    fetchImages();
  };

  // 打开详情模态框
  const handleImageClick = (image, index) => {
    setSelectedImage({ ...image, index });
    setModalVisible(true);
  };

  // 关闭详情模态框
  const handleModalClose = () => {
    setModalVisible(false);
    setSelectedImage(null);
  };

  // 导航到上一张/下一张
  const handleNavigate = (direction) => {
    if (!selectedImage) return;

    const currentIndex = selectedImage.index;
    let newIndex;

    if (direction === 'prev') {
      newIndex = currentIndex > 0 ? currentIndex - 1 : images.length - 1;
    } else {
      newIndex = currentIndex < images.length - 1 ? currentIndex + 1 : 0;
    }

    setSelectedImage({ ...images[newIndex], index: newIndex });
  };

  // 删除图片后刷新
  const handleDelete = () => {
    handleModalClose();
    fetchImages();
  };

  return (
    <div className="w-full h-full">
      <Card>
        {/* 过滤器 */}
        <Space spacing="medium" wrap style={{ marginBottom: '16px' }}>
          <Select
            value={filters.model}
            onChange={(value) => handleFilterChange('model', value)}
            style={{ width: 200 }}
            placeholder={t('选择模型')}
          >
            <Select.Option value="all">{t('全部模型')}</Select.Option>
            <Select.Option value="dall-e-3">DALL-E 3</Select.Option>
            <Select.Option value="dall-e-2">DALL-E 2</Select.Option>
            <Select.Option value="stable-diffusion">
              Stable Diffusion
            </Select.Option>
            <Select.Option value="midjourney">Midjourney</Select.Option>
          </Select>

          <DatePicker
            type="dateRange"
            value={filters.dateRange}
            onChange={(value) => handleFilterChange('dateRange', value)}
            style={{ width: 280 }}
            placeholder={[t('开始日期'), t('结束日期')]}
          />

          <Button
            icon={<IconRefresh />}
            onClick={handleRefresh}
            loading={loading}
          >
            {t('刷新')}
          </Button>
        </Space>

        {/* 图片网格 */}
        <Spin spinning={loading}>
          {images.length === 0 ? (
            <Empty
              image={<IconImage style={{ fontSize: 48, color: '#ccc' }} />}
              title={t('暂无图片')}
              description={t('生成成功的图片将显示在这里')}
              style={{ padding: '60px 0' }}
            />
          ) : (
            <ImageGrid images={images} onImageClick={handleImageClick} />
          )}
        </Spin>

        {/* 分页 */}
        {pagination.total > 0 && (
          <div
            style={{
              marginTop: '24px',
              display: 'flex',
              justifyContent: 'center',
            }}
          >
            <Pagination
              total={pagination.total}
              currentPage={pagination.current}
              pageSize={pagination.pageSize}
              onPageChange={handlePageChange}
              showSizeChanger
              pageSizeOpts={[20, 40, 60, 100]}
              onPageSizeChange={handlePageSizeChange}
            />
          </div>
        )}
      </Card>

      {/* 详情模态框 */}
      {selectedImage && (
        <ImageDetailModal
          visible={modalVisible}
          image={selectedImage}
          onClose={handleModalClose}
          onNavigate={handleNavigate}
          onRegenerate={onRegenerate}
          onDelete={handleDelete}
          hasNext={selectedImage.index < images.length - 1}
          hasPrev={selectedImage.index > 0}
        />
      )}
    </div>
  );
};

export default GalleryView;
