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
import { useTranslation } from 'react-i18next';
import { Button, SideSheet, Spin, Typography } from '@douyinfe/semi-ui';
import { IconCopy, IconDownload } from '@douyinfe/semi-icons';
import { API, copy, showError, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useContainerWidth } from '../../hooks/common/useContainerWidth';

const { Paragraph } = Typography;
const PAGE_SIZE = 24;
const MAX_RENDERED_PAGES = 5;
const FIRST_PAGE_KEY = '__first__';

const Inspiration = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [shellRef, shellWidth] = useContainerWidth();
  const [pageChunks, setPageChunks] = useState([]);
  const [renderStart, setRenderStart] = useState(0);
  const [nextCursor, setNextCursor] = useState('');
  const [hasMore, setHasMore] = useState(true);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [selectedAsset, setSelectedAsset] = useState(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const topSentinelRef = useRef(null);
  const bottomSentinelRef = useRef(null);
  const loadingPagesRef = useRef(new Set());
  const loadSeqRef = useRef(0);
  const detailSeqRef = useRef(0);
  const appendJustHappenedRef = useRef(false);
  const sectionObserversRef = useRef(new Map());
  const sectionHeightsRef = useRef(new Map());
  const [layoutVersion, setLayoutVersion] = useState(0);

  const renderEnd = Math.min(pageChunks.length, renderStart + MAX_RENDERED_PAGES);
  const visiblePages = useMemo(
    () => pageChunks.slice(renderStart, renderEnd),
    [pageChunks, renderEnd, renderStart],
  );

  const renderedAssetCount = useMemo(
    () => visiblePages.reduce((sum, page) => sum + page.items.length, 0),
    [visiblePages],
  );

  const parseJsonObject = (raw) => {
    if (!raw) return {};
    if (typeof raw === 'object') {
      return raw;
    }
    try {
      const parsed = JSON.parse(raw);
      return parsed && typeof parsed === 'object' ? parsed : {};
    } catch (error) {
      return {};
    }
  };

  const selectedParams = useMemo(
    () => parseJsonObject(selectedAsset?.params),
    [selectedAsset?.params],
  );

  const masonryColumnCount = useMemo(() => {
    if (!shellWidth) return 1;
    const maxColumns = Math.max(1, Math.min(6, Math.floor(shellWidth / 320)));
    return Math.max(1, Math.min(renderedAssetCount || 1, maxColumns));
  }, [renderedAssetCount, shellWidth]);

  const estimatePageHeight = useCallback(
    (page) => {
      const itemCount = page?.items?.length || 0;
      if (itemCount === 0) return 0;
      const columns = Math.max(1, masonryColumnCount);
      return Math.max(420, Math.ceil(itemCount / columns) * 320);
    },
    [masonryColumnCount],
  );

  const topSpacerHeight = useMemo(() => {
    return pageChunks
      .slice(0, renderStart)
      .reduce(
        (sum, page) =>
          sum +
          (sectionHeightsRef.current.get(page.key) ?? estimatePageHeight(page)),
        0,
      );
  }, [estimatePageHeight, layoutVersion, pageChunks, renderStart]);

  const bottomSpacerHeight = useMemo(() => {
    return pageChunks
      .slice(renderEnd)
      .reduce(
        (sum, page) =>
          sum +
          (sectionHeightsRef.current.get(page.key) ?? estimatePageHeight(page)),
        0,
      );
  }, [estimatePageHeight, layoutVersion, pageChunks, renderEnd]);

  const registerSectionElement = useCallback((pageKey, element) => {
    const existingObserver = sectionObserversRef.current.get(pageKey);
    if (!element) {
      if (existingObserver) {
        existingObserver.disconnect();
        sectionObserversRef.current.delete(pageKey);
      }
      return;
    }

    if (existingObserver) {
      existingObserver.disconnect();
    }

    const observer = new ResizeObserver((entries) => {
      const height = Math.ceil(entries[0]?.contentRect?.height || 0);
      if (height <= 0) {
        return;
      }
      const previous = sectionHeightsRef.current.get(pageKey);
      if (previous === height) {
        return;
      }
      sectionHeightsRef.current.set(pageKey, height);
      setLayoutVersion((version) => version + 1);
    });

    observer.observe(element);
    sectionObserversRef.current.set(pageKey, observer);
  }, []);

  const formatSeries = useCallback(
    (series) => {
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
    },
    [t],
  );

  const loadAssets = useCallback(
    async (cursor = '', append = false) => {
      const requestSeq = ++loadSeqRef.current;
      const pageKey = cursor || FIRST_PAGE_KEY;
      if (append) {
        setLoadingMore(true);
      } else {
        setLoading(true);
      }
      try {
        const res = await API.get('/api/inspiration/assets', {
          params: { cursor, page_size: PAGE_SIZE },
        });
        if (requestSeq !== loadSeqRef.current) return;
        if (res.data.success) {
          const data = res.data.data || {};
          const items = data.items || [];
          appendJustHappenedRef.current = append;
          setPageChunks((prev) => {
            if (!append) {
              return items.length > 0 ? [{ key: FIRST_PAGE_KEY, items }] : [];
            }
            if (prev.some((page) => page.key === pageKey)) {
              return prev;
            }
            const seen = new Set(
              prev.flatMap((page) => page.items.map((asset) => asset.id)),
            );
            const nextItems = items.filter((asset) => !seen.has(asset.id));
            if (nextItems.length === 0) {
              return prev;
            }
            return [...prev, { key: pageKey, items: nextItems }];
          });
          setNextCursor(data.next_cursor || '');
          setHasMore(data.has_more === true);
        } else {
          showError(res.data.message || t('加载灵感失败'));
        }
      } catch (error) {
        if (requestSeq !== loadSeqRef.current) return;
        showError(error.message || t('加载灵感失败'));
      } finally {
        if (requestSeq !== loadSeqRef.current) return;
        loadingPagesRef.current.delete(pageKey);
        if (append) {
          setLoadingMore(false);
        } else {
          setLoading(false);
        }
      }
    },
    [t],
  );

  useEffect(() => {
    loadAssets('', false);
  }, [loadAssets]);

  useEffect(() => {
    if (pageChunks.length <= MAX_RENDERED_PAGES) {
      if (renderStart !== 0) {
        setRenderStart(0);
      }
      appendJustHappenedRef.current = false;
      return;
    }

    if (appendJustHappenedRef.current) {
      appendJustHappenedRef.current = false;
      setRenderStart(Math.max(0, pageChunks.length - MAX_RENDERED_PAGES));
      return;
    }

    const maxStart = Math.max(0, pageChunks.length - MAX_RENDERED_PAGES);
    if (renderStart > maxStart) {
      setRenderStart(maxStart);
    }
  }, [pageChunks.length, renderStart]);

  useEffect(() => {
    const sentinel = topSentinelRef.current;
    if (!sentinel || renderStart === 0 || loading || loadingMore) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          setRenderStart((current) => Math.max(0, current - 1));
        }
      },
      { rootMargin: '240px 0px' },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [loading, loadingMore, renderStart]);

  useEffect(() => {
    const sentinel = bottomSentinelRef.current;
    if (!sentinel || loading || loadingMore) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (!entries[0]?.isIntersecting) {
          return;
        }

        if (renderEnd < pageChunks.length) {
          setRenderStart((current) => {
            const maxStart = Math.max(0, pageChunks.length - MAX_RENDERED_PAGES);
            return Math.min(maxStart, current + 1);
          });
          return;
        }

        const cursor = nextCursor;
        if (hasMore && cursor && !loadingPagesRef.current.has(cursor)) {
          loadingPagesRef.current.add(cursor);
          loadAssets(cursor, true);
        }
      },
      { rootMargin: '360px 0px' },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasMore, loadAssets, loading, loadingMore, nextCursor, pageChunks.length, renderEnd]);

  useEffect(() => {
    return () => {
      sectionObserversRef.current.forEach((observer) => observer.disconnect());
      sectionObserversRef.current.clear();
    };
  }, []);

  const openDetail = async (asset) => {
    const requestSeq = detailSeqRef.current + 1;
    detailSeqRef.current = requestSeq;
    setSelectedAsset(asset);
    setDetailVisible(true);
    setDetailLoading(true);
    try {
      const res = await API.get(`/api/inspiration/assets/${asset.id}`);
      if (requestSeq !== detailSeqRef.current) {
        return;
      }
      if (res.data.success) {
        setSelectedAsset(res.data.data);
      } else {
        showError(res.data.message || t('加载作品详情失败'));
      }
    } catch (error) {
      if (requestSeq !== detailSeqRef.current) {
        return;
      }
      showError(error.message || t('加载作品详情失败'));
    } finally {
      if (requestSeq === detailSeqRef.current) {
        setDetailLoading(false);
      }
    }
  };

  const copyPrompt = async () => {
    if (!selectedAsset?.prompt) {
      showError(t('暂无提示词'));
      return;
    }
    if (await copy(selectedAsset.prompt)) {
      showSuccess(t('已复制到剪贴板'));
    } else {
      showError(t('复制失败'));
    }
  };

  const downloadAsset = () => {
    if (!selectedAsset?.image_url) return;
    const link = document.createElement('a');
    link.href = selectedAsset.image_url;
    link.download = `inspiration-${selectedAsset.id}.png`;
    link.target = '_blank';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const renderAssetCard = (asset) => (
    <button
      type='button'
      key={asset.id}
      className='inspiration-card'
      style={{ '--inspiration-card-ratio': asset.card_aspect_ratio || 1 }}
      onClick={() => openDetail(asset)}
      aria-label={t('查看详情')}
    >
      <img
        src={asset.thumbnail_url || asset.image_url}
        alt={asset.prompt || 'Generated'}
        loading='lazy'
        decoding='async'
      />
    </button>
  );

  return (
    <div className='inspiration-page'>
      <style>{`
        .inspiration-page {
          min-height: calc(100vh - 60px);
          margin-top: 60px;
          padding: 16px 16px 28px;
          color: var(--semi-color-text-0);
          background: var(--semi-color-bg-0);
        }
        .inspiration-shell {
          width: 100%;
          margin: 0;
          display: flex;
          flex-direction: column;
          gap: 14px;
        }
        .inspiration-masonry {
          column-gap: 4px;
          width: 100%;
        }
        .inspiration-masonry-section {
          width: 100%;
        }
        .inspiration-spacer {
          width: 100%;
          flex-shrink: 0;
        }
        .inspiration-window-sentinel {
          width: 100%;
          height: 1px;
        }
        .inspiration-card {
          position: relative;
          display: block;
          width: 100%;
          break-inside: avoid;
          margin: 0 0 4px;
          padding: 0;
          border: 1px solid var(--semi-color-border);
          border-radius: 10px;
          overflow: hidden;
          background: var(--semi-color-bg-0);
          color: inherit;
          text-align: left;
          cursor: pointer;
          aspect-ratio: var(--inspiration-card-ratio, 1);
          transition: transform 0.18s, border-color 0.18s, box-shadow 0.18s;
        }
        .inspiration-card:hover {
          transform: translateY(-1px);
          border-color: var(--semi-color-primary-light-default);
          box-shadow: 0 18px 42px -30px rgba(15, 23, 42, 0.5);
        }
        .inspiration-card img {
          position: absolute;
          inset: 0;
          display: block;
          width: 100%;
          height: 100%;
          object-fit: cover;
          background: linear-gradient(135deg, rgba(148, 163, 184, 0.1), rgba(148, 163, 184, 0.04));
        }
        .inspiration-load-more {
          display: flex;
          justify-content: center;
          padding: 8px 0 24px;
          min-height: 40px;
        }
        .inspiration-detail {
          display: flex;
          flex-direction: column;
          gap: 16px;
          padding: 16px;
        }
        .inspiration-detail-preview {
          border: 1px solid var(--semi-color-border);
          border-radius: 8px;
          background: var(--semi-color-fill-0);
          overflow: hidden;
          display: flex;
          justify-content: center;
        }
        .inspiration-detail-preview img {
          display: block;
          max-width: 100%;
          max-height: 62vh;
          object-fit: contain;
        }
        .inspiration-detail-actions {
          display: grid;
          grid-template-columns: 1fr 1fr;
          gap: 8px;
        }
        .inspiration-info-block {
          display: flex;
          flex-direction: column;
          gap: 5px;
        }
        .inspiration-info-label {
          color: var(--semi-color-text-2);
          font-size: 12px;
        }
        .inspiration-info-value {
          color: var(--semi-color-text-0);
          font-size: 14px;
          font-weight: 500;
          overflow-wrap: anywhere;
        }
        .inspiration-param-pre {
          margin: 0;
          max-height: 180px;
          overflow: auto;
          padding: 10px;
          border-radius: 8px;
          background: var(--semi-color-fill-0);
          font-size: 12px;
          white-space: pre-wrap;
          overflow-wrap: anywhere;
        }
        @media (max-width: 720px) {
          .inspiration-page {
            padding: 12px 10px 22px;
          }
          .inspiration-masonry { column-gap: 3px; }
          .inspiration-card {
            margin-bottom: 3px;
          }
          .inspiration-detail-actions {
            grid-template-columns: 1fr;
          }
        }
      `}</style>

      <div className='inspiration-shell' ref={shellRef}>
        <Spin spinning={loading}>
          {topSpacerHeight > 0 && (
            <div
              className='inspiration-spacer'
              style={{ height: topSpacerHeight }}
            />
          )}
          <div ref={topSentinelRef} className='inspiration-window-sentinel' />
          {visiblePages.map((page) => (
            <div
              key={page.key}
              className='inspiration-masonry-section'
              ref={(element) => registerSectionElement(page.key, element)}
            >
              <div
                className='inspiration-masonry'
                style={{ columnCount: masonryColumnCount }}
              >
                {page.items.map(renderAssetCard)}
              </div>
            </div>
          ))}
          <div
            ref={bottomSentinelRef}
            className='inspiration-window-sentinel'
          />
          {bottomSpacerHeight > 0 && (
            <div
              className='inspiration-spacer'
              style={{ height: bottomSpacerHeight }}
            />
          )}
          <div className='inspiration-load-more'>
            {loadingMore && <Spin size='small' />}
          </div>
        </Spin>
      </div>

      <SideSheet
        placement='right'
        visible={detailVisible}
        width={isMobile ? '100%' : 520}
        title={t('作品详情')}
        onCancel={() => {
          detailSeqRef.current += 1;
          setDetailVisible(false);
          setDetailLoading(false);
        }}
        bodyStyle={{ padding: 0 }}
      >
        <Spin spinning={detailLoading}>
          {selectedAsset && (
            <div className='inspiration-detail'>
              <div className='inspiration-detail-preview'>
                <img
                  src={selectedAsset.image_url}
                  alt={selectedAsset.prompt || 'Generated'}
                />
              </div>
              <div className='inspiration-detail-actions'>
                <Button
                  theme='outline'
                  type='tertiary'
                  icon={<IconCopy />}
                  onClick={copyPrompt}
                >
                  {t('复制提示词')}
                </Button>
                <Button
                  theme='outline'
                  type='tertiary'
                  icon={<IconDownload />}
                  onClick={downloadAsset}
                >
                  {t('下载图片')}
                </Button>
              </div>
              <div className='inspiration-info-block'>
                <span className='inspiration-info-label'>{t('模型')}</span>
                <span className='inspiration-info-value'>
                  {selectedAsset.display_name || selectedAsset.model_id}
                </span>
              </div>
              <div className='inspiration-info-block'>
                <span className='inspiration-info-label'>{t('模型系列')}</span>
                <span className='inspiration-info-value'>
                  {formatSeries(selectedAsset.model_series)}
                </span>
              </div>
              <div className='inspiration-info-block'>
                <span className='inspiration-info-label'>{t('提示词')}</span>
                <Paragraph
                  copyable={{ content: selectedAsset.prompt || '' }}
                  style={{ margin: 0, whiteSpace: 'pre-wrap' }}
                >
                  {selectedAsset.prompt || '-'}
                </Paragraph>
              </div>
              {Object.keys(selectedParams).length > 0 && (
                <div className='inspiration-info-block'>
                  <span className='inspiration-info-label'>{t('生成参数')}</span>
                  <pre className='inspiration-param-pre'>
                    {JSON.stringify(selectedParams, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          )}
        </Spin>
      </SideSheet>
    </div>
  );
};

export default Inspiration;
