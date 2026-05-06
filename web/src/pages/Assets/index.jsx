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

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import {
  Button,
  Empty,
  Input,
  Select,
  SideSheet,
  Spin,
  Tag,
  Typography,
  Checkbox,
  Popconfirm,
} from '@douyinfe/semi-ui';
import {
  IconChevronUp,
  IconCopy,
  IconCheck,
  IconDownload,
  IconExternalOpen,
  IconFilter,
  IconImage,
  IconRefresh,
  IconSearch,
} from '@douyinfe/semi-icons';
import { API, copy, renderQuota, showError, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Paragraph } = Typography;
const PAGE_SIZE = 24;

const Assets = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const isMobile = useIsMobile();

  const [assets, setAssets] = useState([]);
  const [filters, setFilters] = useState({ models: [], series: [] });
  const [stats, setStats] = useState({
    total_assets: 0,
    latest_created_time: 0,
  });
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [creativeSubmitting, setCreativeSubmitting] = useState(false);
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [batchSubmitting, setBatchSubmitting] = useState(false);
  const [selectedAsset, setSelectedAsset] = useState(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedAssetIds, setSelectedAssetIds] = useState(() => new Set());
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [keyword, setKeyword] = useState('');
  const [submittedKeyword, setSubmittedKeyword] = useState('');
  const [modelId, setModelId] = useState('');
  const [modelSeries, setModelSeries] = useState('');
  const [timeRange, setTimeRange] = useState('');
  const [sortValue, setSortValue] = useState('created_time_desc');

  const sentinelRef = useRef(null);
  const loadingPagesRef = useRef(new Set());
  const loadSeqRef = useRef(0);
  const detailSeqRef = useRef(0);

  const hasMore = assets.length < total;
  const activeFilterCount = [
    submittedKeyword,
    modelId,
    modelSeries,
    timeRange,
  ].filter(Boolean).length;

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

  const parseSortValue = (value) => {
    const parts = (value || 'created_time_desc').split('_');
    const sortOrder = parts.pop() || 'desc';
    return {
      sort_by: parts.join('_') || 'created_time',
      sort_order: sortOrder,
    };
  };

  const selectedParams = useMemo(
    () => parseJsonObject(selectedAsset?.params),
    [selectedAsset?.params],
  );
  const selectedMetadata = useMemo(
    () => parseJsonObject(selectedAsset?.image_metadata),
    [selectedAsset?.image_metadata],
  );

  const getCreativeStatusMeta = useCallback(
    (status) => {
      const statusMap = {
        pending: { label: t('审核中'), color: 'orange' },
        approved: { label: t('已展示'), color: 'green' },
        rejected: { label: t('已驳回'), color: 'red' },
      };
      return statusMap[status || ''] || { label: t('未提交'), color: 'blue' };
    },
    [t],
  );

  const selectedCreativeStatusMeta = useMemo(
    () => getCreativeStatusMeta(selectedAsset?.creative_submission_status),
    [getCreativeStatusMeta, selectedAsset?.creative_submission_status],
  );

  const selectedAssets = useMemo(
    () =>
      assets.filter((asset) => selectedAssetIds.has(asset.task_id || asset.id)),
    [assets, selectedAssetIds],
  );

  const selectedCount = selectedAssetIds.size;
  const hasSelectedAssets = selectedCount > 0;
  const allVisibleSelected =
    assets.length > 0 &&
    assets.every((asset) => selectedAssetIds.has(asset.task_id || asset.id));

  const getTimeRangeParams = useCallback(() => {
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
  }, [timeRange]);

  const buildQueryParams = useCallback(
    (nextPage) => {
      const params = {
        p: nextPage,
        page_size: PAGE_SIZE,
        ...parseSortValue(sortValue),
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
      return params;
    },
    [getTimeRangeParams, modelId, modelSeries, sortValue, submittedKeyword],
  );

  const loadAssets = useCallback(
    async (nextPage = 1, append = false) => {
      const requestSeq = ++loadSeqRef.current;
      if (append) {
        setLoadingMore(true);
      } else {
        setLoading(true);
      }

      try {
        const res = await API.get('/api/image-generation/assets', {
          params: buildQueryParams(nextPage),
        });
        if (requestSeq !== loadSeqRef.current) return;

        if (res.data.success) {
          const data = res.data.data || {};
          const items = data.items || [];
          setAssets((prev) => {
            if (!append) return items;
            const seen = new Set(
              prev.map((asset) => asset.task_id || asset.id),
            );
            const nextItems = items.filter(
              (asset) => !seen.has(asset.task_id || asset.id),
            );
            return [...prev, ...nextItems];
          });
          setTotal(data.total || 0);
          setPage(data.page || nextPage);
          setStats(data.stats || { total_assets: 0, latest_created_time: 0 });
          setFilters(data.filters || { models: [], series: [] });
        } else {
          showError(res.data.message || t('加载资产失败'));
        }
      } catch (error) {
        if (requestSeq !== loadSeqRef.current) return;
        showError(error.message || t('加载资产失败'));
      } finally {
        if (requestSeq !== loadSeqRef.current) return;
        loadingPagesRef.current.delete(nextPage);
        if (append) {
          setLoadingMore(false);
        } else {
          setLoading(false);
        }
      }
    },
    [buildQueryParams, t],
  );

  useEffect(() => {
    loadingPagesRef.current.clear();
    setLoadingMore(false);
    setPage(1);
    loadAssets(1, false);
  }, [loadAssets]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel || !hasMore || loading || loadingMore) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const nextPage = page + 1;
        if (
          entries[0]?.isIntersecting &&
          !loadingPagesRef.current.has(nextPage)
        ) {
          loadingPagesRef.current.add(nextPage);
          loadAssets(nextPage, true);
        }
      },
      { rootMargin: '420px 0px' },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasMore, loadAssets, loading, loadingMore, page]);

  const queueListReload = useCallback(() => {
    loadSeqRef.current += 1;
    loadingPagesRef.current.clear();
    setLoadingMore(false);
    setPage(1);
    setAssets([]);
    setTotal(0);
  }, []);

  const refreshAssets = () => {
    queueListReload();
    loadAssets(1, false);
  };

  const openDetail = async (asset) => {
    const requestSeq = ++detailSeqRef.current;
    setSelectedAsset(asset);
    setDetailVisible(true);
    setDetailLoading(true);
    try {
      const res = await API.get(
        `/api/image-generation/assets/${asset.task_id || asset.id}`,
      );
      if (requestSeq !== detailSeqRef.current) return;
      if (res.data.success) {
        setSelectedAsset(res.data.data);
      } else {
        showError(res.data.message || t('加载资产详情失败'));
      }
    } catch (error) {
      if (requestSeq !== detailSeqRef.current) return;
      showError(error.message || t('加载资产详情失败'));
    } finally {
      if (requestSeq !== detailSeqRef.current) return;
      setDetailLoading(false);
    }
  };

  const toggleAssetSelection = (asset, checked) => {
    const key = asset.task_id || asset.id;
    setSelectedAssetIds((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(key);
      } else {
        next.delete(key);
      }
      return next;
    });
  };

  const selectAllVisibleAssets = () => {
    setSelectedAssetIds((prev) => {
      const next = new Set(prev);
      assets.forEach((asset) => {
        next.add(asset.task_id || asset.id);
      });
      return next;
    });
  };

  const clearSelectedAssets = () => {
    setSelectedAssetIds(new Set());
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

  const downloadSelectedAssets = () => {
    if (!selectedAssets.length) return;
    selectedAssets.forEach((asset, index) => {
      setTimeout(() => downloadAsset(asset), index * 120);
    });
  };

  const deleteSelectedAssets = async () => {
    if (!selectedAssets.length) return;
    setBatchDeleting(true);
    try {
      const results = await Promise.allSettled(
        selectedAssets.map((asset) =>
          API.delete(`/api/image-generation/tasks/${asset.task_id || asset.id}`),
        ),
      );
      const successIds = [];
      const failedCount = results.reduce((count, result, index) => {
        if (result.status === 'fulfilled' && result.value?.data?.success) {
          successIds.push(selectedAssets[index].task_id || selectedAssets[index].id);
          return count;
        }
        return count + 1;
      }, 0);
      if (successIds.length > 0) {
        setAssets((prev) =>
          prev.filter((asset) => !successIds.includes(asset.task_id || asset.id)),
        );
        setTotal((prev) => Math.max(0, prev - successIds.length));
        setSelectedAssetIds((prev) => {
          const next = new Set(prev);
          successIds.forEach((id) => next.delete(id));
          return next;
        });
      }
      if (successIds.length > 0) {
        showSuccess(t('删除成功 {{count}} 个任务', { count: successIds.length }));
      }
      if (failedCount > 0) {
        showError(t('删除失败 {{count}} 个任务', { count: failedCount }));
      }
    } catch (error) {
      showError(error.message || t('删除失败'));
    } finally {
      setBatchDeleting(false);
    }
  };

  const submitSelectedAssetsToCreativeSpace = async () => {
    if (!selectedAssets.length) return;
    setBatchSubmitting(true);
    try {
      const submitableAssets = selectedAssets.filter(
        (asset) => !asset.creative_submission_status,
      );
      if (submitableAssets.length === 0) {
        showError(t('已选择的资产都已提交'));
        return;
      }
      const results = await Promise.allSettled(
        submitableAssets.map((asset) =>
          API.post(
            `/api/image-generation/assets/${asset.task_id || asset.id}/creative-submission`,
          ),
        ),
      );
      let successCount = 0;
      const patchIds = [];
      results.forEach((result, index) => {
        if (result.status === 'fulfilled' && result.value?.data?.success) {
          const submission = result.value.data.data || {};
          const asset = submitableAssets[index];
          const patch = {
            creative_submission_id: submission.id,
            creative_submission_status: submission.status,
            creative_reject_reason: submission.reject_reason || '',
          };
          patchIds.push(asset.task_id || asset.id);
          setSelectedAsset((prev) =>
            prev && (prev.task_id || prev.id) === (asset.task_id || asset.id)
              ? { ...prev, ...patch }
              : prev,
          );
          setAssets((prev) =>
            prev.map((item) =>
              (item.task_id || item.id) === (asset.task_id || asset.id)
                ? { ...item, ...patch }
                : item,
            ),
          );
          successCount += 1;
        }
      });
      if (patchIds.length > 0) {
        setSelectedAssetIds((prev) => {
          const next = new Set(prev);
          patchIds.forEach((id) => next.delete(id));
          return next;
        });
      }
      if (successCount > 0) {
        showSuccess(t('已提交到创意空间审核'));
      }
      if (results.some((result) => result.status === 'rejected')) {
        showError(t('部分提交失败'));
      }
    } catch (error) {
      showError(error.message || t('提交失败'));
    } finally {
      setBatchSubmitting(false);
    }
  };

  const openSourceTask = (asset) => {
    navigate(`/ai-generation?task_id=${asset.task_id || asset.id}`);
  };

  const submitToCreativeSpace = async () => {
    if (!selectedAsset?.task_id || selectedAsset.creative_submission_status) {
      return;
    }
    const taskId = selectedAsset.task_id;
    setCreativeSubmitting(true);
    try {
      const res = await API.post(
        `/api/image-generation/assets/${taskId}/creative-submission`,
      );
      if (res.data.success) {
        const submission = res.data.data || {};
        const patch = {
          creative_submission_id: submission.id,
          creative_submission_status: submission.status,
          creative_reject_reason: submission.reject_reason || '',
        };
        setSelectedAsset((prev) =>
          prev?.task_id === taskId ? { ...prev, ...patch } : prev,
        );
        setAssets((prev) =>
          prev.map((asset) =>
            asset.task_id === taskId ? { ...asset, ...patch } : asset,
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
    const nextKeyword = keyword.trim();
    queueListReload();
    if (nextKeyword === submittedKeyword) {
      loadAssets(1, false);
    } else {
      setSubmittedKeyword(nextKeyword);
    }
  };

  const handleModelSeriesChange = (value) => {
    const nextValue = value || '';
    if (nextValue === modelSeries) return;
    queueListReload();
    setModelSeries(nextValue);
  };

  const handleModelIdChange = (value) => {
    const nextValue = value || '';
    if (nextValue === modelId) return;
    queueListReload();
    setModelId(nextValue);
  };

  const handleTimeRangeChange = (value) => {
    const nextValue = value || '';
    if (nextValue === timeRange) return;
    queueListReload();
    setTimeRange(nextValue);
  };

  const handleSortChange = (value) => {
    const nextValue = value || 'created_time_desc';
    if (nextValue === sortValue) return;
    queueListReload();
    setSortValue(nextValue);
  };

  const resetFilters = () => {
    const alreadyDefault =
      !keyword &&
      !submittedKeyword &&
      !modelId &&
      !modelSeries &&
      !timeRange &&
      sortValue === 'created_time_desc';

    queueListReload();
    setKeyword('');
    setSubmittedKeyword('');
    setModelId('');
    setModelSeries('');
    setTimeRange('');
    setSortValue('created_time_desc');

    if (
      alreadyDefault ||
      (!submittedKeyword && !modelId && !modelSeries && !timeRange)
    ) {
      loadAssets(1, false);
    }
  };

  const scrollToTop = () => {
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const handleCardKeyDown = (event, asset) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      openDetail(asset);
    }
  };

  const renderAssetCard = (asset) => (
    <div
      key={asset.task_id || asset.id}
      className='asset-card'
      role='button'
      tabIndex={0}
      onClick={() => openDetail(asset)}
      onKeyDown={(event) => handleCardKeyDown(event, asset)}
    >
      <div className='asset-card-select'>
        <div onClick={(e) => e.stopPropagation()}>
          <Checkbox
            checked={selectedAssetIds.has(asset.task_id || asset.id)}
            onChange={(e) => toggleAssetSelection(asset, e.target.checked)}
          />
        </div>
      </div>
      <div className='asset-image-wrap'>
        <img src={asset.image_url} alt={asset.prompt || 'Generated'} />
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
          align-items: flex-start;
          justify-content: space-between;
          gap: 16px;
        }
        .assets-title-wrap {
          display: flex;
          flex-direction: column;
          gap: 6px;
          min-width: 0;
        }
        .assets-title-line {
          display: flex;
          align-items: center;
          gap: 10px;
          min-width: 0;
        }
        .assets-title {
          margin: 0;
          color: var(--semi-color-text-0);
          font-size: 20px;
          line-height: 1.3;
          font-weight: 700;
          letter-spacing: 0;
        }
        .assets-filter-count {
          white-space: nowrap;
        }
        .assets-subtitle {
          display: flex;
          align-items: center;
          flex-wrap: wrap;
          gap: 8px;
          color: var(--semi-color-text-2);
          font-size: 13px;
          line-height: 1.4;
        }
        .assets-subtitle strong {
          color: var(--semi-color-text-0);
          font-weight: 650;
        }
        .assets-top-actions {
          display: flex;
          align-items: center;
          justify-content: flex-end;
          gap: 8px;
          flex-wrap: wrap;
          flex-shrink: 0;
        }
        .assets-controls {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-wrap: wrap;
          padding-bottom: 2px;
        }
        .assets-filter-label {
          display: inline-flex;
          align-items: center;
          gap: 6px;
          min-height: 32px;
          color: var(--semi-color-text-2);
          font-size: 13px;
          white-space: nowrap;
        }
        .assets-search {
          width: min(320px, 100%);
          flex: 1 1 260px;
        }
        .assets-select {
          width: 176px;
          flex: 0 1 176px;
        }
        .assets-sort {
          width: 184px;
          flex: 0 1 184px;
        }
        .assets-filter-actions {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-wrap: wrap;
        }
        .assets-masonry {
          column-count: 6;
          column-gap: 4px;
          width: 100%;
        }
        .asset-card {
          position: relative;
          display: inline-block;
          width: 100%;
          break-inside: avoid;
          margin: 0 0 4px;
          overflow: hidden;
          border: 1px solid var(--semi-color-border);
          border-radius: 8px;
          background: var(--semi-color-bg-0);
          color: inherit;
          text-align: left;
          cursor: pointer;
          outline: none;
          transition: border-color 0.18s, transform 0.18s, box-shadow 0.18s;
        }
        .asset-card:hover,
        .asset-card:focus-visible {
          transform: translateY(-1px);
          border-color: var(--semi-color-primary-light-default);
          box-shadow: 0 18px 42px -30px rgba(15, 23, 42, 0.5);
        }
        .asset-image-wrap {
          position: relative;
          width: 100%;
          min-height: 120px;
          background: var(--semi-color-fill-0);
        }
        .asset-card-select {
          position: absolute;
          top: 8px;
          left: 8px;
          z-index: 2;
          display: flex;
          align-items: center;
          justify-content: center;
          padding: 4px;
          border-radius: 8px;
          background: rgba(0, 0, 0, 0.34);
          backdrop-filter: blur(6px);
        }
        .asset-image-wrap img {
          display: block;
          width: 100%;
          height: auto;
        }
        .assets-load-more {
          display: flex;
          justify-content: center;
          align-items: center;
          gap: 8px;
          min-height: 48px;
          padding: 4px 0 18px;
          color: var(--semi-color-text-2);
          font-size: 13px;
        }
        .assets-batch-bar {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 12px;
          padding: 12px 14px;
          border: 1px solid var(--semi-color-border);
          border-radius: 10px;
          background: var(--semi-color-fill-0);
        }
        .assets-batch-meta {
          display: flex;
          align-items: center;
          gap: 10px;
          flex-wrap: wrap;
          min-width: 0;
          color: var(--semi-color-text-1);
          font-size: 13px;
        }
        .assets-batch-actions {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-wrap: wrap;
          justify-content: flex-end;
        }
        .assets-empty-wrap {
          padding: 48px 0;
        }
        .asset-detail {
          display: flex;
          flex-direction: column;
          gap: 16px;
          padding: 16px;
        }
        .asset-detail-preview {
          border: 1px solid var(--semi-color-border);
          border-radius: 8px;
          background: var(--semi-color-fill-0);
          display: flex;
          align-items: center;
          justify-content: center;
          overflow: hidden;
        }
        .asset-detail-preview img {
          display: block;
          max-width: 100%;
          max-height: 62vh;
          object-fit: contain;
        }
        .asset-detail-actions {
          display: grid;
          grid-template-columns: 1fr 1fr;
          gap: 8px;
        }
        .asset-detail-actions .asset-submit-action {
          grid-column: 1 / -1;
        }
        .asset-info-grid {
          display: grid;
          grid-template-columns: 1fr 1fr;
          gap: 12px;
        }
        .asset-info-block {
          display: flex;
          flex-direction: column;
          gap: 5px;
          min-width: 0;
        }
        .asset-info-wide {
          grid-column: 1 / -1;
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
        .asset-code-block {
          margin: 0;
          max-height: 180px;
          overflow: auto;
          padding: 10px;
          border-radius: 8px;
          background: var(--semi-color-fill-0);
          font-size: 12px;
          line-height: 1.5;
          white-space: pre-wrap;
          overflow-wrap: anywhere;
        }
        .asset-reject-reason {
          color: var(--semi-color-danger);
          font-size: 12px;
          line-height: 1.45;
          overflow-wrap: anywhere;
        }
        @media (max-width: 1480px) {
          .assets-masonry { column-count: 5; }
        }
        @media (max-width: 1280px) {
          .assets-masonry { column-count: 4; }
          .assets-select,
          .assets-sort {
            width: 160px;
            flex-basis: 160px;
          }
        }
        @media (max-width: 960px) {
          .assets-topbar {
            flex-direction: column;
            align-items: stretch;
          }
          .assets-top-actions {
            justify-content: flex-start;
          }
          .assets-masonry { column-count: 3; }
          .assets-batch-bar {
            flex-direction: column;
            align-items: stretch;
          }
          .assets-batch-actions {
            justify-content: flex-start;
          }
        }
        @media (max-width: 720px) {
          .assets-page {
            min-height: calc(100vh - 70px);
          }
          .assets-controls {
            align-items: stretch;
          }
          .assets-filter-label {
            width: 100%;
          }
          .assets-search,
          .assets-select,
          .assets-sort {
            width: 100%;
            flex: 1 1 100%;
          }
          .assets-filter-actions,
          .assets-filter-actions .semi-button {
            width: 100%;
          }
          .assets-masonry {
            column-count: 2;
            column-gap: 4px;
          }
          .asset-card {
            margin-bottom: 4px;
          }
          .asset-info-grid {
            grid-template-columns: 1fr;
          }
          .asset-detail-actions {
            grid-template-columns: 1fr;
          }
        }
        @media (max-width: 420px) {
          .assets-masonry { column-count: 1; }
        }
      `}</style>

      <div className='assets-shell'>
        <div className='assets-topbar'>
          <div className='assets-title-wrap'>
            <div className='assets-title-line'>
              <h1 className='assets-title'>{t('资产仓库')}</h1>
              {activeFilterCount > 0 && (
                <Tag color='blue' className='assets-filter-count'>
                  {t('筛选')} {activeFilterCount}
                </Tag>
              )}
            </div>
          </div>
          <div className='assets-top-actions'>
            <Button
              type={hasSelectedAssets ? 'primary' : 'tertiary'}
              icon={<IconCheck />}
              onClick={() => {
                if (allVisibleSelected) {
                  clearSelectedAssets();
                } else {
                  selectAllVisibleAssets();
                }
              }}
              disabled={assets.length === 0}
            >
              {allVisibleSelected
                ? t('取消全选')
                : t('全选')}
            </Button>
            <Button icon={<IconRefresh />} onClick={refreshAssets}>
              {t('刷新')}
            </Button>
            <Button icon={<IconChevronUp />} onClick={scrollToTop}>
              {t('回到顶部')}
            </Button>
            <Button
              type='primary'
              icon={<IconImage />}
              onClick={() => navigate('/ai-generation')}
            >
              {t('去生成图片')}
            </Button>
          </div>
        </div>

        <div className='assets-controls'>
          <span className='assets-filter-label'>
            <IconFilter size='small' />
            {t('筛选')}
          </span>
          <Input
            className='assets-search'
            value={keyword}
            prefix={<IconSearch />}
            placeholder={t('搜索提示词、模型')}
            showClear
            onChange={setKeyword}
            onEnterPress={submitSearch}
          />
          <Select
            className='assets-select'
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
            className='assets-select'
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
            className='assets-select'
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
          <Select
            className='assets-sort'
            value={sortValue}
            onChange={handleSortChange}
          >
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
          <div className='assets-filter-actions'>
            <Button type='primary' icon={<IconSearch />} onClick={submitSearch}>
              {t('查询')}
            </Button>
            <Button icon={<IconRefresh />} onClick={resetFilters}>
              {t('重置')}
            </Button>
          </div>
        </div>

        {hasSelectedAssets && (
          <div className='assets-batch-bar'>
            <div className='assets-batch-meta'>
              <span>
                {t('已选择')} <strong>{selectedCount}</strong> {t('个')}
              </span>
              <Button size='small' type='tertiary' onClick={clearSelectedAssets}>
                {t('取消选择')}
              </Button>
            </div>
            <div className='assets-batch-actions'>
              <Button
                size='small'
                theme='outline'
                type='tertiary'
                icon={<IconDownload />}
                onClick={downloadSelectedAssets}
              >
                {t('下载选中')}
              </Button>
              <Button
                size='small'
                theme='outline'
                type='primary'
                icon={<IconImage />}
                loading={batchSubmitting}
                onClick={submitSelectedAssetsToCreativeSpace}
              >
                {t('发布选中')}
              </Button>
              <Popconfirm
                title={t('确定要删除选中的')}
                content={t('删除后无法恢复，请确认是否继续')}
                okText={t('确认删除')}
                cancelText={t('取消')}
                okType='danger'
                onConfirm={deleteSelectedAssets}
                position='bottom'
              >
                <Button
                  size='small'
                  theme='outline'
                  type='danger'
                  icon={<IconDelete />}
                  loading={batchDeleting}
                >
                  {t('删除选中')}
                </Button>
              </Popconfirm>
            </div>
          </div>
        )}

        <Spin spinning={loading && assets.length === 0}>
          {assets.length > 0 ? (
            <>
              <div className='assets-masonry'>
                {assets.map(renderAssetCard)}
              </div>
              <div ref={sentinelRef} className='assets-load-more'>
                {loadingMore && (
                  <>
                    <Spin size='small' />
                  </>
                )}
                {!hasMore && total > 0 && <span>{t('已加载全部作品')}</span>}
              </div>
            </>
          ) : (
            <div className='assets-empty-wrap'>
              <Empty
                image={<IconImage size='extra-large' />}
                title={t('暂无图片资产')}
                description={t('完成一次图片生成后，成功的图片会出现在这里。')}
              >
                <Button
                  type='primary'
                  icon={<IconImage />}
                  onClick={() => navigate('/ai-generation')}
                >
                  {t('去生成图片')}
                </Button>
              </Empty>
            </div>
          )}
        </Spin>
      </div>

      <SideSheet
        placement='right'
        visible={detailVisible}
        width={isMobile ? '100%' : 560}
        title={t('资产详情')}
        onCancel={() => setDetailVisible(false)}
        bodyStyle={{ padding: 0 }}
      >
        <Spin spinning={detailLoading}>
          {selectedAsset && (
            <div className='asset-detail'>
              <div className='asset-detail-preview'>
                <img
                  src={selectedAsset.image_url}
                  alt={selectedAsset.prompt || 'Generated'}
                />
              </div>

              <div className='asset-detail-actions'>
                <Button
                  className='asset-submit-action'
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

              <div className='asset-info-grid'>
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
                  <span className='asset-info-label'>{t('来源任务')}</span>
                  <span className='asset-info-value'>
                    #{selectedAsset.task_id}
                  </span>
                </div>
                <div className='asset-info-block asset-info-wide'>
                  <span className='asset-info-label'>{t('创意空间')}</span>
                  <span className='asset-info-value'>
                    <Tag color={selectedCreativeStatusMeta.color}>
                      {selectedCreativeStatusMeta.label}
                    </Tag>
                  </span>
                  {selectedAsset.creative_reject_reason && (
                    <span className='asset-reject-reason'>
                      {selectedAsset.creative_reject_reason}
                    </span>
                  )}
                </div>
                <div className='asset-info-block asset-info-wide'>
                  <span className='asset-info-label'>{t('提示词')}</span>
                  <Paragraph
                    copyable={{ content: selectedAsset.prompt || '' }}
                    style={{
                      margin: 0,
                      maxHeight: 160,
                      overflow: 'auto',
                      whiteSpace: 'pre-wrap',
                    }}
                  >
                    {selectedAsset.prompt || '-'}
                  </Paragraph>
                </div>
                {Object.keys(selectedParams).length > 0 && (
                  <div className='asset-info-block asset-info-wide'>
                    <span className='asset-info-label'>{t('生成参数')}</span>
                    <pre className='asset-code-block'>
                      {JSON.stringify(selectedParams, null, 2)}
                    </pre>
                  </div>
                )}
                {Object.keys(selectedMetadata).length > 0 && (
                  <div className='asset-info-block asset-info-wide'>
                    <span className='asset-info-label'>{t('图片元数据')}</span>
                    <pre className='asset-code-block'>
                      {JSON.stringify(selectedMetadata, null, 2)}
                    </pre>
                  </div>
                )}
              </div>
            </div>
          )}
        </Spin>
      </SideSheet>
    </div>
  );
};

export default Assets;
