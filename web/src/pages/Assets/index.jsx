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
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import {
  Button,
  Empty,
  Input,
  Modal,
  Pagination,
  Select,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconCopy,
  IconDownload,
  IconExternalOpen,
  IconImage,
  IconRefresh,
  IconSearch,
} from '@douyinfe/semi-icons';
import { API, copy, renderQuota, showError, showSuccess } from '../../helpers';

const { Paragraph } = Typography;

const Assets = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const [assets, setAssets] = useState([]);
  const [filters, setFilters] = useState({ models: [], series: [] });
  const [stats, setStats] = useState({
    total_assets: 0,
    latest_created_time: 0,
  });
  const [loading, setLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [creativeSubmitting, setCreativeSubmitting] = useState(false);
  const [selectedAsset, setSelectedAsset] = useState(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(24);
  const [total, setTotal] = useState(0);
  const [keyword, setKeyword] = useState('');
  const [submittedKeyword, setSubmittedKeyword] = useState('');
  const [modelId, setModelId] = useState('');
  const [modelSeries, setModelSeries] = useState('');
  const [timeRange, setTimeRange] = useState('');
  const [sortValue, setSortValue] = useState('created_time_desc');

  const formatTime = (timestamp) => {
    if (!timestamp) return '-';
    return dayjs(timestamp * 1000).format('YYYY/MM/DD HH:mm');
  };

  const formatSeries = (series) => {
    if (!series) return t('未分组');
    const seriesMap = {
      openai: 'OpenAI',
      gemini: 'Gemini',
      dalle: 'OpenAI',
      flux: 'Flux',
      midjourney: 'Midjourney',
      'stable-diffusion': 'Stable Diffusion',
    };
    return (
      seriesMap[series.toLowerCase()] ||
      series.charAt(0).toUpperCase() + series.slice(1)
    );
  };

  const parseJsonObject = (raw) => {
    if (!raw) return {};
    try {
      const parsed = JSON.parse(raw);
      return parsed && typeof parsed === 'object' ? parsed : {};
    } catch (e) {
      return {};
    }
  };

  const selectedParams = useMemo(
    () => parseJsonObject(selectedAsset?.params),
    [selectedAsset?.params],
  );
  const selectedMetadata = useMemo(
    () => parseJsonObject(selectedAsset?.image_metadata),
    [selectedAsset?.image_metadata],
  );
  const creativeStatusMeta = useMemo(() => {
    const status = selectedAsset?.creative_submission_status || '';
    const statusMap = {
      pending: { label: t('审核中'), color: 'orange' },
      approved: { label: t('已展示'), color: 'green' },
      rejected: { label: t('已驳回'), color: 'red' },
    };
    return statusMap[status] || { label: t('未提交'), color: 'grey' };
  }, [selectedAsset?.creative_submission_status, t]);

  const getTimeRangeParams = () => {
    const now = dayjs();
    switch (timeRange) {
      case 'today':
        return {
          start_time: now.startOf('day').unix(),
          end_time: now.endOf('day').unix(),
        };
      case 'last7d':
        return {
          start_time: now.subtract(6, 'day').startOf('day').unix(),
          end_time: now.endOf('day').unix(),
        };
      case 'last30d':
        return {
          start_time: now.subtract(29, 'day').startOf('day').unix(),
          end_time: now.endOf('day').unix(),
        };
      case 'thisMonth':
        return {
          start_time: now.startOf('month').unix(),
          end_time: now.endOf('month').unix(),
        };
      default:
        return {};
    }
  };

  const loadAssets = async () => {
    setLoading(true);
    try {
      const [sortBy, sortOrder] = sortValue.split('_').reduce(
        (acc, part, index, parts) => {
          if (index === parts.length - 1) {
            acc[1] = part;
          } else {
            acc[0] = acc[0] ? `${acc[0]}_${part}` : part;
          }
          return acc;
        },
        ['', 'desc'],
      );
      const params = {
        p: page,
        page_size: pageSize,
        sort_by: sortBy || 'created_time',
        sort_order: sortOrder || 'desc',
        ...getTimeRangeParams(),
      };
      if (submittedKeyword.trim()) {
        params.keyword = submittedKeyword.trim();
      }
      if (modelId) {
        params.model_id = modelId;
      }
      if (modelSeries) {
        params.model_series = modelSeries;
      }

      const res = await API.get('/api/image-generation/assets', { params });
      if (res.data.success) {
        const data = res.data.data || {};
        setAssets(data.items || []);
        setTotal(data.total || 0);
        setStats(data.stats || { total_assets: 0, latest_created_time: 0 });
        setFilters(data.filters || { models: [], series: [] });
      } else {
        showError(res.data.message || t('加载资产失败'));
      }
    } catch (error) {
      showError(error.message || t('加载资产失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadAssets();
  }, [
    page,
    pageSize,
    submittedKeyword,
    modelId,
    modelSeries,
    timeRange,
    sortValue,
  ]);

  const openDetail = async (asset) => {
    setSelectedAsset(asset);
    setDetailVisible(true);
    setDetailLoading(true);
    try {
      const res = await API.get(
        `/api/image-generation/assets/${asset.task_id}`,
      );
      if (res.data.success) {
        setSelectedAsset(res.data.data);
      } else {
        showError(res.data.message || t('加载资产详情失败'));
      }
    } catch (error) {
      showError(error.message || t('加载资产详情失败'));
    } finally {
      setDetailLoading(false);
    }
  };

  const copyPrompt = async (asset) => {
    if (!asset?.prompt) {
      showError(t('暂无提示词'));
      return;
    }
    if (await copy(asset.prompt)) {
      showSuccess(t('已复制到剪贴板'));
    } else {
      showError(t('复制失败'));
    }
  };

  const downloadAsset = (asset) => {
    if (!asset?.image_url) return;
    const link = document.createElement('a');
    link.href = asset.image_url;
    link.download = `image-${asset.task_id || asset.id}.png`;
    link.target = '_blank';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const openSourceTask = (asset) => {
    navigate(`/image-generation?task_id=${asset.task_id || asset.id}`);
  };

  const submitToCreativeSpace = async () => {
    if (!selectedAsset?.task_id) return;
    setCreativeSubmitting(true);
    try {
      const res = await API.post(
        `/api/image-generation/assets/${selectedAsset.task_id}/creative-submission`,
      );
      if (res.data.success) {
        const submission = res.data.data || {};
        const nextAsset = {
          ...selectedAsset,
          creative_submission_id: submission.id,
          creative_submission_status: submission.status,
          creative_reject_reason: submission.reject_reason || '',
        };
        setSelectedAsset(nextAsset);
        setAssets((prev) =>
          prev.map((asset) =>
            asset.task_id === selectedAsset.task_id ? nextAsset : asset,
          ),
        );
        showSuccess(t('已提交到创意空间审核'));
      } else {
        showError(res.data.message || t('提交失败'));
      }
    } catch (error) {
      showError(error.message || t('提交失败'));
    } finally {
      setCreativeSubmitting(false);
    }
  };

  const submitSearch = () => {
    setPage(1);
    setSubmittedKeyword(keyword.trim());
  };

  const handleModelSeriesChange = (value) => {
    setPage(1);
    setModelSeries(value || '');
  };

  const handleModelIdChange = (value) => {
    setPage(1);
    setModelId(value || '');
  };

  const handleTimeRangeChange = (value) => {
    setPage(1);
    setTimeRange(value || '');
  };

  const handleSortChange = (value) => {
    setPage(1);
    setSortValue(value || 'created_time_desc');
  };

  const resetFilters = () => {
    setPage(1);
    setKeyword('');
    setSubmittedKeyword('');
    setModelId('');
    setModelSeries('');
    setTimeRange('');
    setSortValue('created_time_desc');
  };

  const renderAssetCard = (asset) => (
    <div
      key={asset.id}
      className='asset-card'
      onClick={() => openDetail(asset)}
    >
      <div className='asset-image-wrap'>
        <img src={asset.image_url} alt={asset.prompt || 'Generated'} />
        <div className='asset-overlay'>
          <div className='asset-actions' onClick={(e) => e.stopPropagation()}>
            <button
              type='button'
              title={t('查看详情')}
              onClick={() => openDetail(asset)}
            >
              <IconImage size='small' />
            </button>
            <button
              type='button'
              title={t('复制提示词')}
              onClick={() => copyPrompt(asset)}
            >
              <IconCopy size='small' />
            </button>
            <button
              type='button'
              title={t('下载图片')}
              onClick={() => downloadAsset(asset)}
            >
              <IconDownload size='small' />
            </button>
            <button
              type='button'
              title={t('打开源任务')}
              onClick={() => openSourceTask(asset)}
            >
              <IconExternalOpen size='small' />
            </button>
          </div>
        </div>
      </div>
      <div className='asset-card-body'>
        <div className='asset-card-title-row'>
          <div className='asset-card-title'>
            {asset.display_name || asset.model_id || t('未知模型')}
          </div>
          {asset.model_series && (
            <Tag size='small' className='asset-series-tag'>
              {formatSeries(asset.model_series)}
            </Tag>
          )}
        </div>
        <div className='asset-card-prompt'>{asset.prompt}</div>
        <div className='asset-card-meta'>
          <span>{formatTime(asset.created_time)}</span>
        </div>
      </div>
    </div>
  );

  return (
    <div className='assets-page'>
      <style>{`
        .assets-page {
          width: 100%;
          min-height: calc(100vh - 112px);
          color: var(--semi-color-text-0);
        }
        .assets-shell {
          width: 100%;
          margin: 0;
          display: flex;
          flex-direction: column;
          gap: 14px;
        }
        .assets-topbar {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 16px;
          min-height: 42px;
        }
        .assets-left-tools {
          display: flex;
          align-items: center;
          gap: 22px;
          min-width: 0;
        }
        .assets-title-chip {
          display: inline-flex;
          align-items: center;
          min-height: 30px;
          padding: 0 14px;
          border-radius: 7px;
          background: var(--semi-color-fill-1);
          color: var(--semi-color-text-0);
          font-size: 13px;
          font-weight: 650;
          white-space: nowrap;
        }
        .assets-right-tools {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-wrap: wrap;
          justify-content: flex-end;
        }
        .assets-search {
          width: 260px;
        }
        .assets-stats-row {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-wrap: wrap;
          margin-top: -6px;
        }
        .assets-stat {
          display: inline-flex;
          align-items: baseline;
          gap: 6px;
          min-height: 30px;
          padding: 0 10px;
          border: 1px solid var(--semi-color-border);
          border-radius: 7px;
          background: var(--semi-color-fill-0);
        }
        .assets-stat-label {
          color: var(--semi-color-text-3);
        }
        .assets-stat-value {
          color: var(--semi-color-text-0);
          font-weight: 650;
          font-size: 13px;
          overflow-wrap: anywhere;
        }
        .assets-toolbar {
          display: grid;
          grid-template-columns: repeat(4, minmax(156px, 220px)) auto;
          gap: 8px;
          align-items: center;
          justify-content: start;
        }
        .assets-masonry {
          column-count: 6;
          column-gap: 14px;
          width: 100%;
        }
        .asset-card {
          display: inline-block;
          width: 100%;
          break-inside: avoid;
          margin: 0 0 14px;
          overflow: hidden;
          border: 1px solid var(--semi-color-border);
          border-radius: 8px;
          background: var(--semi-color-bg-0);
          cursor: pointer;
          transition: border-color 0.18s, transform 0.18s, box-shadow 0.18s;
        }
        .asset-card:hover {
          transform: translateY(-1px);
          border-color: var(--semi-color-primary-light-default);
          box-shadow: 0 18px 42px -30px rgba(15, 23, 42, 0.5);
        }
        .asset-image-wrap {
          position: relative;
          width: 100%;
          background: var(--semi-color-fill-0);
        }
        .asset-image-wrap img {
          display: block;
          width: 100%;
          height: auto;
        }
        .asset-overlay {
          position: absolute;
          inset: 0;
          display: flex;
          align-items: flex-end;
          justify-content: flex-end;
          padding: 10px;
          opacity: 0;
          transition: opacity 0.18s;
          background: linear-gradient(to top, rgba(0,0,0,0.45), transparent 55%);
        }
        .asset-card:hover .asset-overlay {
          opacity: 1;
        }
        .asset-actions {
          display: flex;
          gap: 6px;
        }
        .asset-actions button {
          width: 30px;
          height: 30px;
          border-radius: 7px;
          border: 1px solid rgba(255,255,255,0.16);
          background: rgba(0,0,0,0.52);
          color: #fff;
          cursor: pointer;
          display: inline-flex;
          align-items: center;
          justify-content: center;
          backdrop-filter: blur(8px);
        }
        .asset-card-body {
          padding: 9px 10px 11px;
        }
        .asset-card-title-row {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 8px;
          margin-bottom: 5px;
          min-width: 0;
        }
        .asset-card-title {
          font-size: 13px;
          line-height: 1.35;
          font-weight: 600;
          min-width: 0;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
        .asset-series-tag {
          flex-shrink: 0;
          max-width: 84px;
        }
        .asset-card-prompt {
          font-size: 12px;
          line-height: 1.45;
          color: var(--semi-color-text-2);
          display: -webkit-box;
          -webkit-line-clamp: 2;
          -webkit-box-orient: vertical;
          overflow: hidden;
          min-height: 34px;
          overflow-wrap: anywhere;
        }
        .asset-card-meta {
          margin-top: 11px;
          display: flex;
          justify-content: space-between;
          align-items: center;
          gap: 8px;
          color: var(--semi-color-text-3);
          font-size: 12px;
        }
        .assets-pagination {
          display: flex;
          justify-content: center;
          padding: 4px 0 12px;
        }
        .asset-detail-body {
          display: grid;
          grid-template-columns: minmax(0, 1fr) 300px;
          gap: 18px;
          padding: 18px;
        }
        .asset-detail-preview {
          min-height: 460px;
          border: 1px solid var(--semi-color-border);
          border-radius: 8px;
          background: var(--semi-color-fill-0);
          display: flex;
          align-items: center;
          justify-content: center;
          overflow: hidden;
        }
        .asset-detail-preview img {
          max-width: 100%;
          max-height: 72vh;
          object-fit: contain;
          display: block;
        }
        .asset-detail-side {
          display: flex;
          flex-direction: column;
          gap: 14px;
          min-width: 0;
        }
        .asset-info-block {
          display: flex;
          flex-direction: column;
          gap: 4px;
        }
        .asset-info-label {
          color: var(--semi-color-text-2);
          font-size: 12px;
        }
        .asset-info-value {
          color: var(--semi-color-text-0);
          font-size: 14px;
          font-weight: 500;
          overflow-wrap: anywhere;
        }
        .asset-detail-actions {
          margin-top: auto;
          display: flex;
          flex-direction: column;
          gap: 8px;
        }
        @media (max-width: 1480px) {
          .assets-masonry {
            column-count: 5;
          }
        }
        @media (max-width: 1280px) {
          .assets-masonry {
            column-count: 4;
          }
          .assets-toolbar {
            grid-template-columns: repeat(3, minmax(0, 1fr));
          }
          .assets-search {
            width: 220px;
          }
        }
        @media (max-width: 960px) {
          .assets-topbar {
            flex-direction: column;
            align-items: stretch;
          }
          .assets-left-tools,
          .assets-right-tools {
            width: 100%;
            justify-content: space-between;
          }
          .assets-search {
            width: min(100%, 320px);
          }
          .assets-masonry {
            column-count: 3;
          }
          .asset-detail-body {
            grid-template-columns: 1fr;
          }
        }
        @media (max-width: 720px) {
          .assets-page {
            min-height: calc(100vh - 70px);
          }
          .assets-left-tools {
            align-items: flex-start;
            flex-direction: column;
            gap: 10px;
          }
          .assets-right-tools {
            align-items: stretch;
            flex-direction: column;
          }
          .assets-search {
            width: 100%;
          }
          .assets-toolbar {
            grid-template-columns: 1fr;
          }
          .assets-masonry {
            column-count: 2;
            column-gap: 10px;
          }
          .asset-card {
            margin-bottom: 10px;
          }
        }
        @media (max-width: 420px) {
          .assets-masonry {
            column-count: 1;
          }
        }
      `}</style>

      <div className='assets-shell'>
        <div className='assets-topbar'>
          <div className='assets-left-tools'>
            <span className='assets-title-chip'>{t('资产仓库')}</span>
          </div>
          <div className='assets-right-tools'>
            <Input
              className='assets-search'
              value={keyword}
              prefix={<IconSearch />}
              placeholder={t('搜索提示词、模型')}
              showClear
              onChange={setKeyword}
              onEnterPress={submitSearch}
            />
            <Button
              type='tertiary'
              onClick={() => navigate('/image-generation')}
            >
              {t('去生成图片')}
            </Button>
          </div>
        </div>

        <div className='assets-stats-row'>
          <span className='assets-stat'>
            <span className='assets-stat-label'>{t('资产总数')}</span>
            <span className='assets-stat-value'>
              {stats.total_assets || total}
            </span>
          </span>
          <span className='assets-stat'>
            <span className='assets-stat-label'>{t('最近生成')}</span>
            <span className='assets-stat-value'>
              {formatTime(stats.latest_created_time)}
            </span>
          </span>
        </div>

        <div className='assets-toolbar'>
          <Select
            value={modelSeries}
            onChange={handleModelSeriesChange}
            placeholder={t('全部系列')}
            showClear
          >
            {(filters.series || []).map((item) => (
              <Select.Option key={item.model_series} value={item.model_series}>
                {formatSeries(item.model_series)}
              </Select.Option>
            ))}
          </Select>
          <Select
            value={modelId}
            onChange={handleModelIdChange}
            placeholder={t('全部模型')}
            showClear
            filter
          >
            {(filters.models || []).map((item) => (
              <Select.Option key={item.model_id} value={item.model_id}>
                {item.display_name || item.model_id}
              </Select.Option>
            ))}
          </Select>
          <Select
            value={timeRange}
            onChange={handleTimeRangeChange}
            placeholder={t('全部时间')}
            showClear
          >
            <Select.Option value='today'>{t('今天')}</Select.Option>
            <Select.Option value='last7d'>{t('近 7 天')}</Select.Option>
            <Select.Option value='last30d'>{t('近 30 天')}</Select.Option>
            <Select.Option value='thisMonth'>{t('本月')}</Select.Option>
          </Select>
          <Select value={sortValue} onChange={handleSortChange}>
            <Select.Option value='created_time_desc'>
              {t('创建时间倒序')}
            </Select.Option>
            <Select.Option value='created_time_asc'>
              {t('创建时间正序')}
            </Select.Option>
            <Select.Option value='completed_time_desc'>
              {t('完成时间倒序')}
            </Select.Option>
            <Select.Option value='cost_desc'>{t('消耗额度倒序')}</Select.Option>
          </Select>
          <div style={{ display: 'flex', gap: 8 }}>
            <Button type='primary' icon={<IconSearch />} onClick={submitSearch}>
              {t('查询')}
            </Button>
            <Button icon={<IconRefresh />} onClick={resetFilters}>
              {t('重置')}
            </Button>
          </div>
        </div>

        <Spin spinning={loading}>
          {assets.length > 0 ? (
            <>
              <div className='assets-masonry'>
                {assets.map(renderAssetCard)}
              </div>
              {total > pageSize && (
                <div className='assets-pagination'>
                  <Pagination
                    total={total}
                    currentPage={page}
                    pageSize={pageSize}
                    onPageChange={setPage}
                    showSizeChanger
                    onPageSizeChange={(size) => {
                      setPageSize(size);
                      setPage(1);
                    }}
                    pageSizeOpts={[12, 24, 48, 96]}
                  />
                </div>
              )}
            </>
          ) : (
            <Empty
              image={<IconImage size='extra-large' />}
              title={t('暂无图片资产')}
              description={t('完成一次图片生成后，成功的图片会出现在这里。')}
            >
              <Button
                type='primary'
                onClick={() => navigate('/image-generation')}
              >
                {t('去生成图片')}
              </Button>
            </Empty>
          )}
        </Spin>
      </div>

      <Modal
        visible={detailVisible}
        title={t('资产详情')}
        footer={null}
        width={1080}
        onCancel={() => setDetailVisible(false)}
        bodyStyle={{ padding: 0 }}
      >
        <Spin spinning={detailLoading}>
          {selectedAsset && (
            <div className='asset-detail-body'>
              <div className='asset-detail-preview'>
                <img
                  src={selectedAsset.image_url}
                  alt={selectedAsset.prompt || 'Generated'}
                />
              </div>
              <div className='asset-detail-side'>
                <div className='asset-info-block'>
                  <span className='asset-info-label'>{t('模型')}</span>
                  <span className='asset-info-value'>
                    {selectedAsset.display_name || selectedAsset.model_id}
                  </span>
                </div>
                <div className='asset-info-block'>
                  <span className='asset-info-label'>{t('模型系列')}</span>
                  <span className='asset-info-value'>
                    {formatSeries(selectedAsset.model_series)}
                  </span>
                </div>
                <div className='asset-info-block'>
                  <span className='asset-info-label'>{t('创建时间')}</span>
                  <span className='asset-info-value'>
                    {formatTime(selectedAsset.created_time)}
                  </span>
                </div>
                <div className='asset-info-block'>
                  <span className='asset-info-label'>{t('完成时间')}</span>
                  <span className='asset-info-value'>
                    {formatTime(selectedAsset.completed_time)}
                  </span>
                </div>
                <div className='asset-info-block'>
                  <span className='asset-info-label'>{t('消耗额度')}</span>
                  <span className='asset-info-value'>
                    {renderQuota(selectedAsset.cost || 0)}
                  </span>
                </div>
                <div className='asset-info-block'>
                  <span className='asset-info-label'>{t('提示词')}</span>
                  <Paragraph
                    copyable={{ content: selectedAsset.prompt || '' }}
                    style={{
                      margin: 0,
                      maxHeight: 120,
                      overflow: 'auto',
                      whiteSpace: 'pre-wrap',
                    }}
                  >
                    {selectedAsset.prompt || '-'}
                  </Paragraph>
                </div>
                {Object.keys(selectedParams).length > 0 && (
                  <div className='asset-info-block'>
                    <span className='asset-info-label'>{t('生成参数')}</span>
                    <pre
                      style={{
                        margin: 0,
                        maxHeight: 120,
                        overflow: 'auto',
                        padding: 10,
                        borderRadius: 8,
                        background: 'var(--semi-color-fill-0)',
                        fontSize: 12,
                        whiteSpace: 'pre-wrap',
                      }}
                    >
                      {JSON.stringify(selectedParams, null, 2)}
                    </pre>
                  </div>
                )}
                {Object.keys(selectedMetadata).length > 0 && (
                  <div className='asset-info-block'>
                    <span className='asset-info-label'>{t('图片元数据')}</span>
                    <pre
                      style={{
                        margin: 0,
                        maxHeight: 120,
                        overflow: 'auto',
                        padding: 10,
                        borderRadius: 8,
                        background: 'var(--semi-color-fill-0)',
                        fontSize: 12,
                        whiteSpace: 'pre-wrap',
                      }}
                    >
                      {JSON.stringify(selectedMetadata, null, 2)}
                    </pre>
                  </div>
                )}
                <div className='asset-info-block'>
                  <span className='asset-info-label'>{t('来源任务')}</span>
                  <span className='asset-info-value'>
                    #{selectedAsset.task_id}
                  </span>
                </div>
                <div className='asset-info-block'>
                  <span className='asset-info-label'>{t('创意空间')}</span>
                  <span className='asset-info-value'>
                    <Tag color={creativeStatusMeta.color}>
                      {creativeStatusMeta.label}
                    </Tag>
                  </span>
                  {selectedAsset.creative_reject_reason && (
                    <span className='asset-info-label'>
                      {selectedAsset.creative_reject_reason}
                    </span>
                  )}
                </div>
                <div className='asset-detail-actions'>
                  <Button
                    type='primary'
                    icon={<IconImage />}
                    loading={creativeSubmitting}
                    disabled={Boolean(selectedAsset.creative_submission_status)}
                    onClick={submitToCreativeSpace}
                  >
                    {t('提交到创意空间')}
                  </Button>
                  <Button
                    theme='outline'
                    type='tertiary'
                    icon={<IconCopy />}
                    onClick={() => copyPrompt(selectedAsset)}
                  >
                    {t('复制提示词')}
                  </Button>
                  <Button
                    theme='outline'
                    type='tertiary'
                    icon={<IconDownload />}
                    onClick={() => downloadAsset(selectedAsset)}
                  >
                    {t('下载图片')}
                  </Button>
                  <Button
                    theme='outline'
                    type='tertiary'
                    icon={<IconExternalOpen />}
                    onClick={() => openSourceTask(selectedAsset)}
                  >
                    {t('打开源任务')}
                  </Button>
                </div>
              </div>
            </div>
          )}
        </Spin>
      </Modal>
    </div>
  );
};

export default Assets;
