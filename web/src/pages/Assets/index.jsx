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

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { Button, Checkbox, Empty, Input, Popconfirm, Select, SideSheet, Spin, Tag, Typography } from '@douyinfe/semi-ui';
import {
  IconArrowLeft,
  IconChevronUp,
  IconCopy,
  IconDelete,
  IconDownload,
  IconExternalOpen,
  IconFilter,
  IconImage,
  IconRefresh,
  IconSearch,
  IconTick,
  IconVideo,
} from '@douyinfe/semi-icons';
import { API, copy, renderQuota, showError, showSuccess } from '../../helpers';
import { formatModelSeriesLabel } from '../../helpers/modelSeries';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useContainerWidth } from '../../hooks/common/useContainerWidth';
import PlayableVideo from '../../components/PlayableVideo';
import VideoGenerationTaskCard from '../../components/VideoGenerationTaskCard';

const { Paragraph } = Typography;
const PAGE_SIZE = 24;
const MEDIA_IMAGE = 'image';
const MEDIA_VIDEO = 'video';

const EMPTY_FILTERS = { models: [], series: [] };
const EMPTY_STATS = { total_assets: 0, latest_created_time: 0 };

const Assets = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const isMobile = useIsMobile();
  const [shellRef, shellWidth] = useContainerWidth();

  const [activeMediaType, setActiveMediaType] = useState(MEDIA_IMAGE);
  const [imageState, setImageState] = useState({
    items: [],
    filters: EMPTY_FILTERS,
    stats: EMPTY_STATS,
    loading: false,
    loadingMore: false,
    page: 1,
    total: 0,
    hasMore: false,
    keyword: '',
    submittedKeyword: '',
    modelId: '',
    modelSeries: '',
    timeRange: '',
    sortValue: 'created_time_desc',
    selectedIds: new Set(),
  });
  const [videoState, setVideoState] = useState({
    items: [],
    filters: EMPTY_FILTERS,
    stats: EMPTY_STATS,
    loading: false,
    loadingMore: false,
    page: 1,
    total: 0,
    hasMore: false,
    nextCursor: '',
    keyword: '',
    submittedKeyword: '',
    modelId: '',
    modelSeries: '',
    timeRange: '',
    sortValue: 'created_time_desc',
    selectedIds: new Set(),
  });
  const [detailVisible, setDetailVisible] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [selectedAsset, setSelectedAsset] = useState(null);
  const [batchDeleting, setBatchDeleting] = useState(false);

  const imageSentinelRef = useRef(null);
  const videoSentinelRef = useRef(null);
  const imageLoadSeqRef = useRef(0);
  const videoLoadSeqRef = useRef(0);
  const detailSeqRef = useRef(0);
  const loadingPagesRef = useRef(new Set());
  const videoFiltersLoadedRef = useRef(false);

  const activeState = activeMediaType === MEDIA_IMAGE ? imageState : videoState;
  const setActiveState =
    activeMediaType === MEDIA_IMAGE ? setImageState : setVideoState;
  const sentinelRef = activeMediaType === MEDIA_IMAGE ? imageSentinelRef : videoSentinelRef;

  const formatTime = useCallback((timestamp) => {
    if (!timestamp) return '-';
    return dayjs(timestamp * 1000).format('YYYY/MM/DD HH:mm');
  }, []);

  const parseSortValue = useCallback((value) => {
    const normalized = value || 'created_time_desc';
    const parts = normalized.split('_');
    const sortOrder = parts.pop() || 'desc';
    return {
      sort_by: parts.join('_') || 'created_time',
      sort_order: sortOrder,
    };
  }, []);

  const formatSeries = useCallback(
    (series) => {
      if (!series) return t('未分组');
      return formatModelSeriesLabel(series, t('未分组'));
    },
    [t],
  );

  const activeFilterCount = [
    activeState.submittedKeyword,
    activeState.modelId,
    activeState.modelSeries,
    activeState.timeRange,
  ].filter(Boolean).length;

  const visibleAssetIds = useMemo(
    () => activeState.items.map((asset) => asset.assetKey),
    [activeState.items],
  );

  const selectedCount = activeState.selectedIds.size;
  const hasSelectedAssets = selectedCount > 0;
  const visibleSelectedCount = useMemo(
    () =>
      visibleAssetIds.reduce(
        (count, id) => count + (activeState.selectedIds.has(id) ? 1 : 0),
        0,
      ),
    [activeState.selectedIds, visibleAssetIds],
  );
  const allVisibleSelected =
    visibleAssetIds.length > 0 &&
    visibleSelectedCount === visibleAssetIds.length;
  const partiallyVisibleSelected =
    visibleSelectedCount > 0 && !allVisibleSelected;

  const currentDetailAsset = selectedAsset;

  const getTimeRangeParams = useCallback((timeRange) => {
    if (!timeRange) {
      return {};
    }
    const now = dayjs();
    if (timeRange === 'today') {
      return {
        start_time: now.startOf('day').unix(),
        end_time: now.endOf('day').unix(),
      };
    }
    if (timeRange === 'last7d') {
      return {
        start_time: now.subtract(6, 'day').startOf('day').unix(),
        end_time: now.endOf('day').unix(),
      };
    }
    if (timeRange === 'last30d') {
      return {
        start_time: now.subtract(29, 'day').startOf('day').unix(),
        end_time: now.endOf('day').unix(),
      };
    }
    if (timeRange === 'thisMonth') {
      return {
        start_time: now.startOf('month').unix(),
        end_time: now.endOf('month').unix(),
      };
    }
    return {};
  }, []);

  const buildImageParams = useCallback(
    (queryState, nextPage) => {
      const params = {
        p: nextPage,
        page_size: PAGE_SIZE,
        ...parseSortValue(queryState.sortValue),
        ...getTimeRangeParams(queryState.timeRange),
      };
      if (queryState.submittedKeyword.trim()) {
        params.keyword = queryState.submittedKeyword.trim();
      }
      if (queryState.modelId) params.model_id = queryState.modelId;
      if (queryState.modelSeries) params.model_series = queryState.modelSeries;
      return params;
    },
    [getTimeRangeParams, parseSortValue],
  );

  const buildVideoParams = useCallback(
    (queryState, nextPage) => {
      const params = {
        p: nextPage,
        page_size: PAGE_SIZE,
        status: 'completed',
        ...parseSortValue(queryState.sortValue),
        ...getTimeRangeParams(queryState.timeRange),
      };
      if (queryState.submittedKeyword.trim()) {
        params.keyword = queryState.submittedKeyword.trim();
      }
      if (queryState.modelId) params.model_id = queryState.modelId;
      if (queryState.modelSeries) params.model_series = queryState.modelSeries;
      return params;
    },
    [getTimeRangeParams, parseSortValue],
  );

  const loadImageAssets = useCallback(
    async (nextPage = 1, append = false, queryState = imageState) => {
      const requestSeq = ++imageLoadSeqRef.current;
      setImageState((prev) => ({
        ...prev,
        loading: !append,
        loadingMore: append,
      }));
      const params = buildImageParams(queryState, nextPage);

      try {
        const res = await API.get('/api/image-generation/assets', { params });
        if (requestSeq !== imageLoadSeqRef.current) return;
        if (res.data.success) {
          const data = res.data.data || {};
          const items = (data.items || []).map((item) => ({
            ...item,
            mediaType: MEDIA_IMAGE,
            assetKey: item.task_id || item.id,
          }));
          setImageState((prev) => ({
            ...prev,
            items: append ? [...prev.items, ...items] : items,
            filters: data.filters || EMPTY_FILTERS,
            stats: data.stats || EMPTY_STATS,
            page: data.page || nextPage,
            total: data.total || 0,
            hasMore:
              typeof data.total === 'number'
                ? (append ? prev.items.length : 0) + items.length < data.total
                : false,
            loading: false,
            loadingMore: false,
          }));
        } else {
          showError(res.data.message || t('加载资产失败'));
        }
      } catch (error) {
        if (requestSeq !== imageLoadSeqRef.current) return;
        showError(error.message || t('加载资产失败'));
        setImageState((prev) => ({
          ...prev,
          loading: false,
          loadingMore: false,
        }));
      } finally {
        loadingPagesRef.current.delete(`image:${nextPage}`);
      }
    },
    [buildImageParams, imageState, t],
  );

  const loadVideoAssets = useCallback(
    async (nextPage = 1, append = false, queryState = videoState) => {
      const requestSeq = ++videoLoadSeqRef.current;
      setVideoState((prev) => ({
        ...prev,
        loading: !append,
        loadingMore: append,
      }));
      const params = buildVideoParams(queryState, nextPage);

      try {
        const res = await API.get('/api/video-generation/tasks', { params });
        if (requestSeq !== videoLoadSeqRef.current) return;
        if (res.data.success) {
          const data = res.data.data || {};
          const items = (data.items || []).map((item) => ({
            ...item,
            mediaType: MEDIA_VIDEO,
            assetKey: item.id,
          }));
          setVideoState((prev) => ({
            ...prev,
            items: append ? [...prev.items, ...items] : items,
            filters: data.filters || EMPTY_FILTERS,
            stats: data.stats || EMPTY_STATS,
            page: data.page || nextPage,
            total: data.total || 0,
            nextCursor: data.next_cursor || '',
            hasMore: Boolean(data.has_more),
            loading: false,
            loadingMore: false,
          }));
        } else {
          showError(res.data.message || t('加载视频失败'));
        }
      } catch (error) {
        if (requestSeq !== videoLoadSeqRef.current) return;
        showError(error.message || t('加载视频失败'));
        setVideoState((prev) => ({
          ...prev,
          loading: false,
          loadingMore: false,
        }));
      } finally {
        loadingPagesRef.current.delete(`video:${nextPage}`);
      }
    },
    [buildVideoParams, t, videoState],
  );

  const loadVideoFilters = useCallback(async () => {
    if (videoFiltersLoadedRef.current) {
      return;
    }
    try {
      const res = await API.get('/api/video-generation/models');
      if (!res.data.success) {
        return;
      }
      const items = res.data.data || [];
      const modelMap = new Map();
      const seriesMap = new Map();
      items.forEach((item) => {
        const modelId = String(item.request_model || '').trim();
        if (modelId && !modelMap.has(modelId)) {
          modelMap.set(modelId, {
            model_id: modelId,
            display_name: String(item.display_name || modelId).trim(),
          });
        }
        const modelSeries = String(item.model_series || '').trim();
        if (modelSeries && !seriesMap.has(modelSeries)) {
          seriesMap.set(modelSeries, {
            model_series: modelSeries,
            display_name: String(item.display_name || modelSeries).trim(),
          });
        }
      });
      const filters = {
        models: Array.from(modelMap.values()).sort((a, b) =>
          a.display_name.localeCompare(b.display_name),
        ),
        series: Array.from(seriesMap.values()).sort((a, b) =>
          a.model_series.localeCompare(b.model_series),
        ),
      };
      videoFiltersLoadedRef.current = true;
      setVideoState((prev) => ({
        ...prev,
        filters,
      }));
    } catch (error) {
      // Ignore filter preload errors; page can still render the list itself.
    }
  }, []);

  const refreshActiveMedia = useCallback(() => {
    if (activeMediaType === MEDIA_IMAGE) {
      imageLoadSeqRef.current += 1;
      loadingPagesRef.current.clear();
      setImageState((prev) => ({
        ...prev,
        items: [],
        total: 0,
        page: 1,
        hasMore: false,
      }));
      loadImageAssets(1, false);
      return;
    }
    videoLoadSeqRef.current += 1;
    loadingPagesRef.current.clear();
    setVideoState((prev) => ({
      ...prev,
      items: [],
      total: 0,
      page: 1,
      nextCursor: '',
      hasMore: false,
    }));
    loadVideoAssets(1, false);
  }, [activeMediaType, loadImageAssets, loadVideoAssets]);

  useEffect(() => {
    loadImageAssets(1, false);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (activeMediaType === MEDIA_IMAGE) {
      if (
        imageState.items.length === 0 &&
        !imageState.loading &&
        imageLoadSeqRef.current === 0
      ) {
        loadImageAssets(imageState.page || 1, false);
      }
      return;
    }
    if (videoState.items.length === 0 && !videoState.loading) {
      loadVideoFilters();
      loadVideoAssets(videoState.page || 1, false);
    }
  }, [
    activeMediaType,
    imageState.items.length,
    imageState.loading,
    imageState.page,
    loadImageAssets,
    loadVideoFilters,
    loadVideoAssets,
    videoState.items.length,
    videoState.loading,
    videoState.page,
  ]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel) return undefined;
    if (!activeState.hasMore || activeState.loading || activeState.loadingMore) {
      return undefined;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        const nextPage = activeState.page + 1;
        if (
          entries[0]?.isIntersecting &&
          !loadingPagesRef.current.has(`${activeMediaType}:${nextPage}`)
        ) {
          loadingPagesRef.current.add(`${activeMediaType}:${nextPage}`);
          if (activeMediaType === MEDIA_IMAGE) {
            loadImageAssets(nextPage, true);
          } else {
            loadVideoAssets(nextPage, true);
          }
        }
      },
      { rootMargin: '420px 0px' },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [
    activeMediaType,
    activeState.hasMore,
    activeState.items,
    activeState.loading,
    activeState.loadingMore,
    activeState.page,
    loadImageAssets,
    loadVideoAssets,
  ]);

  const openDetail = useCallback(async (asset) => {
    if (!asset?.assetKey) return;
    const requestSeq = ++detailSeqRef.current;
    setSelectedAsset(asset);
    setDetailVisible(true);
    setDetailLoading(true);

    try {
      const url =
        asset.mediaType === MEDIA_VIDEO
          ? `/api/video-generation/tasks/${asset.assetKey}`
          : `/api/image-generation/assets/${asset.assetKey}`;
      const res = await API.get(url);
      if (requestSeq !== detailSeqRef.current) return;
      if (res.data.success) {
        setSelectedAsset({
          ...(res.data.data || asset),
          mediaType: asset.mediaType,
          assetKey: asset.assetKey,
        });
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
  }, [t]);

  const closeDetail = useCallback(() => {
    setDetailVisible(false);
  }, []);

  const getActiveTaskId = useCallback((asset) => asset?.task_id || asset?.id, []);

  const handleSelectAsset = useCallback((asset, checked) => {
    const key = asset.assetKey;
    setActiveState((prev) => {
      const next = new Set(prev.selectedIds);
      if (checked) {
        next.add(key);
      } else {
        next.delete(key);
      }
      return { ...prev, selectedIds: next };
    });
  }, [setActiveState]);

  const toggleAllVisibleAssets = useCallback(() => {
    setActiveState((prev) => {
      const next = new Set(prev.selectedIds);
      const shouldSelect = visibleAssetIds.some((id) => !next.has(id));
      visibleAssetIds.forEach((id) => {
        if (shouldSelect) next.add(id);
        else next.delete(id);
      });
      return { ...prev, selectedIds: next };
    });
  }, [setActiveState, visibleAssetIds]);

  const clearSelectedAssets = useCallback(() => {
    setActiveState((prev) => ({ ...prev, selectedIds: new Set() }));
  }, [setActiveState]);

  const copyPrompt = useCallback(
    async (asset) => {
      if (!asset?.prompt) {
        showError(t('暂无提示词'));
        return;
      }
      if (await copy(asset.prompt)) {
        showSuccess(t('已复制到剪贴板'));
      } else {
        showError(t('复制失败'));
      }
    },
    [t],
  );

  const downloadAsset = useCallback((asset) => {
    const url = asset?.mediaType === MEDIA_VIDEO ? asset?.video_url || asset?.result_url : asset?.image_url;
    if (!url) return;
    const link = document.createElement('a');
    link.href = url;
    link.download = `${asset.mediaType}-${asset.assetKey}`;
    link.target = '_blank';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }, []);

  const deleteSelectedAssets = useCallback(async () => {
    const selectedAssets = activeState.items.filter((asset) => activeState.selectedIds.has(asset.assetKey));
    if (!selectedAssets.length) return;

    setBatchDeleting(true);
    try {
      const results = await Promise.allSettled(
        selectedAssets.map((asset) => {
          const url =
            asset.mediaType === MEDIA_VIDEO
              ? `/api/video-generation/tasks/${asset.assetKey}`
              : `/api/image-generation/tasks/${asset.assetKey}`;
          return API.delete(url);
        }),
      );
      const successKeys = [];
      const failedCount = results.reduce((count, result, index) => {
        if (result.status === 'fulfilled' && result.value?.data?.success) {
          successKeys.push(selectedAssets[index].assetKey);
          return count;
        }
        return count + 1;
      }, 0);
      if (successKeys.length > 0) {
        setActiveState((prev) => ({
          ...prev,
          items: prev.items.filter((asset) => !successKeys.includes(asset.assetKey)),
          total: Math.max(0, prev.total - successKeys.length),
          selectedIds: (() => {
            const next = new Set(prev.selectedIds);
            successKeys.forEach((key) => next.delete(key));
            return next;
          })(),
        }));
        showSuccess(t('删除成功 {{count}} 个任务', { count: successKeys.length }));
      }
      if (failedCount > 0) {
        showError(t('删除失败 {{count}} 个任务', { count: failedCount }));
      }
    } catch (error) {
      showError(error.message || t('删除失败'));
    } finally {
      setBatchDeleting(false);
    }
  }, [activeState.items, activeState.selectedIds, setActiveState, t]);

  const handleVideoRetry = useCallback(
    async (asset) => {
      if (!asset?.assetKey) return;
      try {
        const res = await API.post(`/api/video-generation/tasks/${asset.assetKey}/retry`);
        if (res.data.success) {
          showSuccess(t('已创建新的重试任务'));
          refreshActiveMedia();
        } else {
          showError(res.data.message || t('重试失败'));
        }
      } catch (error) {
        showError(error.message || t('重试失败'));
      }
    },
    [refreshActiveMedia, t],
  );

  const openSourceTask = useCallback((asset) => {
    if (!asset?.assetKey) return;
    if (asset.mediaType === MEDIA_VIDEO) {
      navigate(`/canvas?task_id=${asset.assetKey}&mode=video`);
      return;
    }
    navigate(`/canvas?task_id=${asset.assetKey}`);
  }, [navigate]);

  const openTaskLogs = useCallback(() => {
    navigate('/console/log');
  }, [navigate]);

  const handleMediaTypeSwitch = useCallback(
    (mediaType) => {
      if (mediaType === activeMediaType) return;
      setActiveMediaType(mediaType);
    },
    [activeMediaType],
  );

  const submitSearch = useCallback(() => {
    if (activeMediaType === MEDIA_IMAGE) {
      setImageState((prev) => {
        const next = {
          ...prev,
          submittedKeyword: prev.keyword.trim(),
          items: [],
          total: 0,
          page: 1,
          hasMore: false,
        };
        imageLoadSeqRef.current += 1;
        loadImageAssets(1, false, next);
        return next;
      });
      return;
    }
    setVideoState((prev) => {
      const next = {
        ...prev,
        submittedKeyword: prev.keyword.trim(),
        items: [],
        total: 0,
        page: 1,
        hasMore: false,
        nextCursor: '',
      };
      videoLoadSeqRef.current += 1;
      loadVideoAssets(1, false, next);
      return next;
    });
  }, [activeMediaType, loadImageAssets, loadVideoAssets, setActiveState]);

  const resetFilters = useCallback(() => {
    if (activeMediaType === MEDIA_IMAGE) {
      setImageState((prev) => {
        const next = {
          ...prev,
          keyword: '',
          submittedKeyword: '',
          modelId: '',
          modelSeries: '',
          timeRange: '',
          sortValue: 'created_time_desc',
          items: [],
          total: 0,
          page: 1,
          hasMore: false,
          selectedIds: new Set(),
        };
        imageLoadSeqRef.current += 1;
        loadImageAssets(1, false, next);
        return next;
      });
      return;
    }
    setVideoState((prev) => {
      const next = {
        ...prev,
        keyword: '',
        submittedKeyword: '',
        modelId: '',
        modelSeries: '',
        timeRange: '',
        sortValue: 'created_time_desc',
        items: [],
        total: 0,
        page: 1,
        hasMore: false,
        nextCursor: '',
        selectedIds: new Set(),
      };
      videoLoadSeqRef.current += 1;
      loadVideoAssets(1, false, next);
      return next;
    });
  }, [activeMediaType, loadImageAssets, loadVideoAssets, setActiveState]);

  const handleFieldChange = useCallback((field, value) => {
    setActiveState((prev) => ({ ...prev, [field]: value || '' }));
  }, [setActiveState]);

  const handleSortChange = useCallback((value) => {
    setActiveState((prev) => ({ ...prev, sortValue: value || 'created_time_desc' }));
  }, [setActiveState]);

  const renderImageCard = (asset) => (
    <button
      key={asset.assetKey}
      type='button'
      className='asset-card asset-card-image'
      onClick={() => openDetail(asset)}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          openDetail(asset);
        }
      }}
    >
      <div className='asset-card-select' onClick={(e) => e.stopPropagation()}>
        <Checkbox
          checked={activeState.selectedIds.has(asset.assetKey)}
          onChange={(e) => handleSelectAsset(asset, e.target.checked)}
        />
      </div>
      <div className='asset-media asset-media-image'>
        <img src={asset.thumbnail_url || asset.image_url} alt={asset.prompt || 'Generated'} loading='lazy' />
      </div>
      <div className='asset-card-overlay asset-card-overlay-compact'>
        <div className='asset-card-meta asset-card-meta-compact'>
          <span className='asset-card-model'>{asset.display_name || asset.model_id || '-'}</span>
          <span className='asset-card-time'>
            {asset.completed_time ? formatTime(asset.completed_time) : formatTime(asset.created_time)}
          </span>
        </div>
      </div>
    </button>
  );

  const renderVideoCard = (asset) => (
    <div key={asset.assetKey} className='asset-video-card-wrap'>
      <button
        type='button'
        className='asset-card asset-card-video'
        onClick={() => openDetail(asset)}
      >
        <div className='asset-card-select' onClick={(e) => e.stopPropagation()}>
          <Checkbox
            checked={activeState.selectedIds.has(asset.assetKey)}
            onChange={(e) => handleSelectAsset(asset, e.target.checked)}
          />
        </div>
        <div className='asset-video-media'>
          {asset.thumbnail_url ? (
            <img
              src={asset.thumbnail_url}
              alt={asset.display_name || asset.model_id || 'Video'}
              loading='lazy'
            />
          ) : (
            <div className='asset-video-placeholder'>
              <IconVideo size='extra-large' />
            </div>
          )}
        </div>
        <div className='asset-card-overlay asset-card-overlay-compact'>
          <div className='asset-card-meta asset-card-meta-compact'>
            <span className='asset-card-model'>{asset.display_name || asset.model_id || '-'}</span>
            <span className='asset-card-time'>
              {asset.completed_time ? formatTime(asset.completed_time) : formatTime(asset.created_time)}
            </span>
          </div>
        </div>
      </button>
    </div>
  );

  const renderDetailPanel = () => {
    if (!currentDetailAsset) return null;
    const isVideo = currentDetailAsset.mediaType === MEDIA_VIDEO;
    const detailTitle = isVideo ? t('视频资产详情') : t('资产详情');

    return (
      <SideSheet
        placement='right'
        visible={detailVisible}
        width={isMobile ? '100%' : '100%'}
        bodyStyle={{ padding: 0, background: 'var(--semi-color-bg-0)' }}
        style={{ width: '100vw', maxWidth: '100vw' }}
        title={detailTitle}
        onCancel={closeDetail}
      >
        <Spin spinning={detailLoading}>
          <div className='asset-detail-shell'>
            <div className='asset-detail-topbar'>
              <Button type='tertiary' icon={<IconArrowLeft />} onClick={closeDetail}>
                {t('返回')}
              </Button>
              <div className='asset-detail-top-actions'>
                {isVideo ? (
                  <Button type='primary' icon={<IconExternalOpen />} onClick={() => openTaskLogs(currentDetailAsset)}>
                    {t('查看任务日志')}
                  </Button>
                ) : null}
                <Button icon={<IconDownload />} onClick={() => downloadAsset(currentDetailAsset)}>
                  {isVideo ? t('下载视频') : t('下载图片')}
                </Button>
              </div>
            </div>
            <div className='asset-detail-grid'>
              <div className='asset-detail-preview'>
                {isVideo ? (
                  <PlayableVideo
                    task={currentDetailAsset}
                    poster={currentDetailAsset.thumbnail_url || ''}
                    style={{ width: '100%', height: '100%', maxHeight: '72vh', objectFit: 'contain' }}
                  />
                ) : currentDetailAsset.image_url ? (
                  <img
                    src={currentDetailAsset.image_url}
                    alt={currentDetailAsset.prompt || 'Generated'}
                    className='asset-detail-image'
                  />
                ) : (
                  <div className='asset-detail-empty'>
                    {t('暂无媒体预览')}
                  </div>
                )}
              </div>

              <div className='asset-detail-info'>
                <div className='asset-detail-info-header'>
                  <div className='asset-detail-info-title'>
                    {currentDetailAsset.prompt || t('暂无提示词')}
                  </div>
                  <Tag color={isVideo ? 'blue' : 'green'}>
                    {isVideo ? t('视频') : t('图片')}
                  </Tag>
                </div>

                <div className='asset-detail-info-actions'>
                  <Button theme='outline' type='tertiary' icon={<IconCopy />} onClick={() => copyPrompt(currentDetailAsset)}>
                    {t('复制提示词')}
                  </Button>
                  <Button theme='outline' type='tertiary' icon={<IconExternalOpen />} onClick={() => openSourceTask(currentDetailAsset)}>
                    {t('打开源任务')}
                  </Button>
                  {isVideo ? (
                    <Button
                      theme='outline'
                      type='tertiary'
                      icon={<IconExternalOpen />}
                      onClick={() => {
                        const target = currentDetailAsset.result_url || currentDetailAsset.video_url;
                        if (target) {
                          window.open(target, '_blank', 'noopener');
                        } else {
                          showError(t('暂无可打开视频'));
                        }
                      }}
                    >
                      {t('打开视频')}
                    </Button>
                  ) : null}
                  {isVideo && currentDetailAsset.status === 'failed' ? (
                    <Button theme='outline' type='tertiary' icon={<IconRefresh />} onClick={() => handleVideoRetry(currentDetailAsset)}>
                      {t('重试')}
                    </Button>
                  ) : null}
                </div>

                <div className='asset-detail-grid-list'>
                  <div><span>{t('模型')}</span><strong>{currentDetailAsset.display_name || currentDetailAsset.model_id || '-'}</strong></div>
                  <div><span>{t('模型系列')}</span><strong>{formatSeries(currentDetailAsset.model_series)}</strong></div>
                  <div><span>{t('创建时间')}</span><strong>{formatTime(currentDetailAsset.created_time)}</strong></div>
                  <div><span>{t('完成时间')}</span><strong>{formatTime(currentDetailAsset.completed_time)}</strong></div>
                  <div><span>{isVideo ? t('任务 ID') : t('来源任务')}</span><strong>#{getActiveTaskId(currentDetailAsset)}</strong></div>
                  <div><span>{isVideo ? t('时长') : t('消耗额度')}</span><strong>{isVideo ? (currentDetailAsset.duration ? `${currentDetailAsset.duration}s` : '-') : renderQuota(currentDetailAsset.cost || 0)}</strong></div>
                  {isVideo ? (
                    <>
                      <div><span>{t('分辨率')}</span><strong>{currentDetailAsset.resolution || '-'}</strong></div>
                      <div><span>{t('比例')}</span><strong>{currentDetailAsset.aspect_ratio || '-'}</strong></div>
                      <div><span>{t('消耗额度')}</span><strong>{renderQuota(currentDetailAsset.quota || 0)}</strong></div>
                      <div><span>{t('请求类型')}</span><strong>{currentDetailAsset.request_type || '-'}</strong></div>
                      <div className='asset-detail-wide'><span>{t('状态')}</span><strong>{currentDetailAsset.status || '-'}</strong></div>
                      {currentDetailAsset.fail_reason ? (
                        <div className='asset-detail-wide'><span>{t('失败原因')}</span><strong>{currentDetailAsset.fail_reason}</strong></div>
                      ) : null}
                    </>
                  ) : (
                    <div className='asset-detail-wide'>
                      <span>{t('说明')}</span>
                      <strong>{t('已完成的图片作品会集中展示在这里。')}</strong>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        </Spin>
      </SideSheet>
    );
  };

  const renderToolbar = () => (
    <div className='assets-toolbar'>
      <div className='assets-toolbar-main'>
        <div className='assets-toolbar-tabs'>
          <button
            type='button'
            className={activeMediaType === MEDIA_IMAGE ? 'assets-tab assets-tab-active' : 'assets-tab'}
            onClick={() => handleMediaTypeSwitch(MEDIA_IMAGE)}
          >
            {t('图片')}
          </button>
          <button
            type='button'
            className={activeMediaType === MEDIA_VIDEO ? 'assets-tab assets-tab-active' : 'assets-tab'}
            onClick={() => handleMediaTypeSwitch(MEDIA_VIDEO)}
          >
            {t('视频')}
          </button>
        </div>
        <div className='assets-toolbar-copy'>
          <div className='assets-toolbar-title-line'>
            <h1 className='assets-page-title'>{t('资产仓库')}</h1>
            {activeFilterCount > 0 ? <Tag color='blue'>{t('筛选')} {activeFilterCount}</Tag> : null}
          </div>
          <div className='assets-toolbar-subtitle'>
            {activeMediaType === MEDIA_VIDEO ? t('已归档的视频作品会集中展示在这里。') : t('已完成的图片作品会集中展示在这里。')}
            <strong>{t('共 {{count}} 项', { count: activeState.total || activeState.items.length })}</strong>
          </div>
        </div>
      </div>
      <div className='assets-toolbar-actions'>
        <Button type={hasSelectedAssets ? 'primary' : 'tertiary'} icon={<IconTick />} onClick={toggleAllVisibleAssets} disabled={activeState.items.length === 0}>
          {allVisibleSelected ? t('取消全选') : t('全选当前页')}
        </Button>
        <Button icon={<IconRefresh />} onClick={refreshActiveMedia}>{t('刷新')}</Button>
        <Button icon={<IconChevronUp />} onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}>{t('回到顶部')}</Button>
        <Button type='primary' icon={activeMediaType === MEDIA_VIDEO ? <IconVideo /> : <IconImage />} onClick={() => navigate('/canvas')}>
          {activeMediaType === MEDIA_VIDEO ? t('去生成视频') : t('去生成图片')}
        </Button>
      </div>
    </div>
  );

  const renderFilters = () => (
    <div className='assets-controls'>
      <span className='assets-filter-label'>
        <IconFilter size='small' />
        {t('筛选')}
      </span>
      <Input
        className='assets-search'
        value={activeState.keyword}
        prefix={<IconSearch />}
        placeholder={activeMediaType === MEDIA_VIDEO ? t('搜索提示词、模型') : t('搜索提示词、模型')}
        showClear
        onChange={(value) => handleFieldChange('keyword', value)}
        onEnterPress={submitSearch}
      />
      <Select
        className='assets-select'
        value={activeState.modelSeries}
        onChange={(value) => handleFieldChange('modelSeries', value)}
        placeholder={t('全部系列')}
        showClear
      >
        {(activeState.filters.series || []).map((item) => (
          <Select.Option key={item.model_series} value={item.model_series}>
            {formatSeries(item.model_series)}
          </Select.Option>
        ))}
      </Select>
      <Select
        className='assets-select'
        value={activeState.modelId}
        onChange={(value) => handleFieldChange('modelId', value)}
        placeholder={t('全部模型')}
        showClear
        filter
      >
        {(activeState.filters.models || []).map((item) => (
          <Select.Option key={item.model_id} value={item.model_id}>
            {item.display_name || item.model_id}
          </Select.Option>
        ))}
      </Select>
      <Select
        className='assets-select'
        value={activeState.timeRange}
        onChange={(value) => handleFieldChange('timeRange', value)}
        placeholder={t('全部时间')}
        showClear
      >
        <Select.Option value='today'>{t('今天')}</Select.Option>
        <Select.Option value='last7d'>{t('近 7 天')}</Select.Option>
        <Select.Option value='last30d'>{t('近 30 天')}</Select.Option>
        <Select.Option value='thisMonth'>{t('本月')}</Select.Option>
      </Select>
      <Select className='assets-sort' value={activeState.sortValue} onChange={handleSortChange}>
        <Select.Option value='created_time_desc'>{t('创建时间倒序')}</Select.Option>
        <Select.Option value='created_time_asc'>{t('创建时间正序')}</Select.Option>
        <Select.Option value='completed_time_desc'>{t('完成时间倒序')}</Select.Option>
        <Select.Option value='cost_desc'>{t('消耗额度倒序')}</Select.Option>
      </Select>
      <div className='assets-filter-actions'>
        <Button type='primary' icon={<IconSearch />} onClick={submitSearch}>{t('查询')}</Button>
        <Button icon={<IconRefresh />} onClick={resetFilters}>{t('重置')}</Button>
      </div>
      <div className='assets-selection-actions'>
        <Checkbox checked={allVisibleSelected} indeterminate={partiallyVisibleSelected} disabled={activeState.items.length === 0} onChange={toggleAllVisibleAssets}>
          {allVisibleSelected ? t('取消全选') : t('全选当前页')}
        </Checkbox>
        <Button size='small' type='tertiary' disabled={!hasSelectedAssets} onClick={clearSelectedAssets}>
          {t('清空已选')}
        </Button>
        <Tag color={hasSelectedAssets ? 'blue' : 'grey'}>{t('已选 {{count}} 项', { count: selectedCount })}</Tag>
      </div>
    </div>
  );

  const renderBatchBar = () =>
    hasSelectedAssets ? (
      <div className='assets-batch-bar'>
        <div className='assets-batch-meta'>
          <span>
            {t('已选择')} <strong>{selectedCount}</strong> {t('个')}
          </span>
          <Button size='small' type='tertiary' onClick={clearSelectedAssets}>{t('取消选择')}</Button>
        </div>
        <div className='assets-batch-actions'>
          <Button size='small' theme='outline' type='tertiary' icon={<IconDownload />} onClick={() => activeState.items.filter((asset) => activeState.selectedIds.has(asset.assetKey)).forEach((asset, index) => setTimeout(() => downloadAsset(asset), index * 120))}>
            {t('下载选中')}
          </Button>
          <Popconfirm title={t('确定要删除选中的')} content={t('删除后无法恢复，请确认是否继续')} okText={t('确认删除')} cancelText={t('取消')} okType='danger' onConfirm={deleteSelectedAssets} position='bottom'>
            <Button size='small' theme='outline' type='danger' icon={<IconDelete />} loading={batchDeleting}>
              {t('删除选中')}
            </Button>
          </Popconfirm>
        </div>
      </div>
    ) : null;

  const renderMediaList = () => {
    if (activeState.loading && activeState.items.length === 0) {
      return <div className='assets-loading'><Spin /></div>;
    }
    if (!activeState.items.length) {
      return (
        <div className='assets-empty-wrap'>
          <Empty
            image={activeMediaType === MEDIA_VIDEO ? <IconVideo size='extra-large' /> : <IconImage size='extra-large' />}
            title={activeMediaType === MEDIA_VIDEO ? t('暂无视频资产') : t('暂无图片资产')}
            description={activeMediaType === MEDIA_VIDEO ? t('完成一次视频生成后，成功的视频会出现在这里。') : t('完成一次图片生成后，成功的图片会出现在这里。')}
          >
            <Button type='primary' icon={<IconImage />} onClick={() => navigate('/canvas')}>
              {activeMediaType === MEDIA_VIDEO ? t('去生成视频') : t('去生成图片')}
            </Button>
          </Empty>
        </div>
      );
    }

    return (
      <>
        <div className={activeMediaType === MEDIA_VIDEO ? 'assets-grid assets-grid-video' : 'assets-grid assets-grid-image'}>
          {activeState.items.map((asset) =>
            activeMediaType === MEDIA_VIDEO ? renderVideoCard(asset) : renderImageCard(asset),
          )}
        </div>
        <div ref={sentinelRef} className='assets-load-more'>
          {activeState.loadingMore && <Spin size='small' />}
          {!activeState.hasMore && activeState.total > 0 && <span>{t('已加载全部作品')}</span>}
        </div>
      </>
    );
  };

  return (
    <div className='assets-page'>
      <style>{`
        .assets-page {
          width: 100%;
          min-height: calc(100vh - 112px);
          padding-top: 40px;
          box-sizing: border-box;
          color: var(--semi-color-text-0);
        }
        .assets-shell {
          width: 100%;
          margin: 0;
          display: flex;
          flex-direction: column;
          gap: 14px;
        }
        .assets-toolbar {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 16px;
          flex-wrap: wrap;
        }
        .assets-toolbar-main {
          display: flex;
          align-items: center;
          gap: 14px;
          flex-wrap: wrap;
        }
        .assets-toolbar-copy {
          display: flex;
          flex-direction: column;
          gap: 6px;
          min-width: 0;
        }
        .assets-toolbar-title-line {
          display: flex;
          align-items: center;
          gap: 10px;
          min-width: 0;
        }
        .assets-page-title {
          margin: 0;
          font-size: 20px;
          line-height: 1.3;
          font-weight: 700;
        }
        .assets-toolbar-subtitle {
          display: flex;
          align-items: center;
          flex-wrap: wrap;
          gap: 8px;
          color: var(--semi-color-text-2);
          font-size: 13px;
        }
        .assets-toolbar-subtitle strong {
          color: var(--semi-color-text-0);
        }
        .assets-toolbar-tabs {
          display: inline-flex;
          gap: 8px;
          padding: 4px;
          border-radius: 14px;
          background: var(--semi-color-fill-0);
          border: 1px solid var(--semi-color-border);
        }
        .assets-tab {
          border: 0;
          background: transparent;
          padding: 8px 14px;
          border-radius: 10px;
          cursor: pointer;
          color: var(--semi-color-text-1);
        }
        .assets-tab-active {
          background: var(--semi-color-bg-0);
          color: var(--semi-color-primary);
          box-shadow: 0 4px 16px rgba(15, 23, 42, 0.06);
        }
        .assets-toolbar-actions {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-wrap: wrap;
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
        .assets-filter-actions, .assets-selection-actions {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-wrap: wrap;
        }
        .assets-batch-bar {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 12px;
          padding: 12px 14px;
          border: 1px solid var(--semi-color-border);
          border-radius: 12px;
          background: var(--semi-color-fill-0);
        }
        .assets-batch-meta {
          display: flex;
          align-items: center;
          gap: 10px;
          flex-wrap: wrap;
        }
        .assets-batch-actions {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-wrap: wrap;
          justify-content: flex-end;
        }
        .assets-grid {
          display: grid;
          width: 100%;
          gap: 12px;
          justify-content: flex-start;
        }
        .assets-grid-image {
          grid-template-columns: repeat(auto-fill, 220px);
        }
        .assets-grid-video {
          grid-template-columns: repeat(auto-fill, 220px);
        }
        .asset-video-card-wrap {
          width: 100%;
        }
        .asset-card {
          position: relative;
          display: block;
          width: 100%;
          overflow: hidden;
          border: 0;
          border-radius: 16px;
          background: var(--semi-color-bg-0);
          cursor: pointer;
          padding: 0;
          text-align: left;
          box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06);
          transition: transform 0.18s ease, box-shadow 0.18s ease;
        }
        .asset-card-image {
          height: 284px;
        }
        .asset-card-video {
          height: 184px;
        }
        .asset-card:hover {
          transform: translateY(-2px);
          box-shadow: 0 18px 42px -30px rgba(15, 23, 42, 0.42);
        }
        .asset-card-select {
          position: absolute;
          top: 10px;
          left: 10px;
          z-index: 2;
          display: flex;
          align-items: center;
          justify-content: center;
          padding: 4px;
          border-radius: 10px;
          background: rgba(15, 23, 42, 0.34);
          backdrop-filter: blur(6px);
        }
        .asset-media {
          width: 100%;
          background: var(--semi-color-fill-0);
        }
        .asset-media img,
        .asset-video-media img {
          display: block;
          width: 100%;
          height: 100%;
          object-fit: cover;
        }
        .asset-media-image {
          height: 100%;
        }
        .asset-video-media {
          position: relative;
          width: 100%;
          height: 100%;
          overflow: hidden;
          background: var(--semi-color-fill-0);
        }
        .asset-video-placeholder {
          width: 100%;
          height: 100%;
          display: flex;
          align-items: center;
          justify-content: center;
          color: var(--semi-color-text-2);
        }
        .asset-card-overlay {
          position: absolute;
          left: 0;
          right: 0;
          bottom: 0;
          padding: 24px 12px 10px;
          background: linear-gradient(180deg, rgba(15, 23, 42, 0), rgba(15, 23, 42, 0.82));
          color: #fff;
        }
        .asset-card-overlay-compact {
          padding-top: 32px;
        }
        .asset-card-meta {
          display: flex;
          justify-content: space-between;
          gap: 8px;
          font-size: 12px;
          opacity: 0.8;
          margin-top: 4px;
        }
        .asset-card-meta-compact {
          flex-direction: column;
          align-items: flex-start;
          gap: 2px;
          margin-top: 0;
        }
        .asset-card-model,
        .asset-card-time {
          max-width: 100%;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
        .asset-card-model {
          font-size: 13px;
          font-weight: 600;
          line-height: 1.4;
        }
        .asset-card-time {
          font-size: 12px;
          line-height: 1.35;
        }
        .asset-loading {
          padding: 48px 0;
          display: flex;
          justify-content: center;
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
        .assets-empty-wrap {
          padding: 48px 0;
        }
        .asset-detail-shell {
          width: 100%;
          min-height: 100vh;
          padding: 18px;
          box-sizing: border-box;
          display: flex;
          flex-direction: column;
          gap: 16px;
        }
        .asset-detail-topbar {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 12px;
          flex-wrap: wrap;
        }
        .asset-detail-top-actions {
          display: flex;
          gap: 8px;
          flex-wrap: wrap;
        }
        .asset-detail-grid {
          display: grid;
          grid-template-columns: minmax(0, 1.45fr) minmax(320px, 0.7fr);
          gap: 18px;
          align-items: start;
        }
        .asset-detail-preview {
          min-height: 68vh;
          border-radius: 18px;
          overflow: hidden;
          background: var(--semi-color-fill-0);
          display: flex;
          align-items: center;
          justify-content: center;
        }
        .asset-detail-image {
          display: block;
          width: 100%;
          max-height: 72vh;
          object-fit: contain;
          background: var(--semi-color-fill-0);
        }
        .asset-detail-empty {
          padding: 56px 0;
          color: var(--semi-color-text-2);
        }
        .asset-detail-info {
          display: flex;
          flex-direction: column;
          gap: 14px;
          padding: 4px 0;
        }
        .asset-detail-info-header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 12px;
        }
        .asset-detail-info-title {
          font-size: 18px;
          font-weight: 700;
          line-height: 1.4;
          color: var(--semi-color-text-0);
          overflow-wrap: anywhere;
        }
        .asset-detail-info-actions {
          display: flex;
          gap: 8px;
          flex-wrap: wrap;
        }
        .asset-detail-grid-list {
          display: grid;
          grid-template-columns: repeat(2, minmax(0, 1fr));
          gap: 12px;
        }
        .asset-detail-grid-list > div {
          display: flex;
          flex-direction: column;
          gap: 4px;
          padding: 12px;
          border-radius: 14px;
          background: var(--semi-color-fill-0);
        }
        .asset-detail-grid-list span {
          color: var(--semi-color-text-2);
          font-size: 12px;
        }
        .asset-detail-grid-list strong {
          font-size: 14px;
          color: var(--semi-color-text-0);
          font-weight: 600;
          overflow-wrap: anywhere;
        }
        .asset-detail-wide {
          grid-column: 1 / -1;
        }
        .asset-detail-grid-list pre {
          margin: 0;
          white-space: pre-wrap;
          overflow-wrap: anywhere;
          font-size: 12px;
          line-height: 1.5;
        }
        @media (max-width: 960px) {
          .asset-detail-grid {
            grid-template-columns: 1fr;
          }
        }
        @media (max-width: 720px) {
          .assets-page {
            min-height: calc(100vh - 70px);
            padding-top: 24px;
          }
          .assets-search,
          .assets-select,
          .assets-sort {
            width: 100%;
            flex: 1 1 100%;
          }
          .assets-grid-image,
          .assets-grid-video {
            grid-template-columns: repeat(2, minmax(0, 1fr));
          }
          .asset-card-image {
            height: 220px;
          }
          .asset-card-video {
            height: 154px;
          }
          .asset-detail-shell {
            padding: 12px;
          }
          .asset-detail-grid-list {
            grid-template-columns: 1fr;
          }
        }
      `}</style>

      <div className='assets-shell' ref={shellRef}>
        {renderToolbar()}
        {renderFilters()}
        {renderBatchBar()}
        {renderMediaList()}
      </div>

      {renderDetailPanel()}
    </div>
  );
};

export default Assets;
