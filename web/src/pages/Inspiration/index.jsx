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

const Inspiration = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [shellRef, shellWidth] = useContainerWidth();
  const [assets, setAssets] = useState([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [selectedAsset, setSelectedAsset] = useState(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const sentinelRef = useRef(null);
  const loadingPagesRef = useRef(new Set());
  const loadSeqRef = useRef(0);

  const hasMore = assets.length < total;

  const parseJsonObject = (raw) => {
    if (!raw) return {};
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
    return Math.max(1, Math.min(assets.length || 1, maxColumns));
  }, [assets.length, shellWidth]);

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
    async (nextPage = 1, append = false) => {
      const requestSeq = ++loadSeqRef.current;
      if (append) {
        setLoadingMore(true);
      } else {
        setLoading(true);
      }
      try {
        const res = await API.get('/api/inspiration/assets', {
          params: { p: nextPage, page_size: PAGE_SIZE },
        });
        if (requestSeq !== loadSeqRef.current) return;
        if (res.data.success) {
          const data = res.data.data || {};
          const items = data.items || [];
          setAssets((prev) => {
            if (!append) return items;
            const seen = new Set(prev.map((asset) => asset.id));
            const nextItems = items.filter((asset) => !seen.has(asset.id));
            return [...prev, ...nextItems];
          });
          setTotal(data.total || 0);
          setPage(data.page || nextPage);
        } else {
          showError(res.data.message || t('加载灵感失败'));
        }
      } catch (error) {
        if (requestSeq !== loadSeqRef.current) return;
        showError(error.message || t('加载灵感失败'));
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
    [t],
  );

  useEffect(() => {
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
      { rootMargin: '360px 0px' },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasMore, loadAssets, loading, loadingMore, page]);

  const openDetail = async (asset) => {
    setSelectedAsset(asset);
    setDetailVisible(true);
    setDetailLoading(true);
    try {
      const res = await API.get(`/api/inspiration/assets/${asset.id}`);
      if (res.data.success) {
        setSelectedAsset(res.data.data);
      } else {
        showError(res.data.message || t('加载作品详情失败'));
      }
    } catch (error) {
      showError(error.message || t('加载作品详情失败'));
    } finally {
      setDetailLoading(false);
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
      onClick={() => openDetail(asset)}
      aria-label={t('查看详情')}
    >
      <img
        src={asset.thumbnail_url || asset.image_url}
        alt={asset.prompt || 'Generated'}
        loading='lazy'
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
          column-gap: 14px;
          width: 100%;
        }
        .inspiration-card {
          display: inline-block;
          width: 100%;
          break-inside: avoid;
          margin: 0 0 14px;
          padding: 0;
          border: 1px solid var(--semi-color-border);
          border-radius: 10px;
          overflow: hidden;
          background: var(--semi-color-bg-0);
          color: inherit;
          text-align: left;
          cursor: pointer;
          transition: transform 0.18s, border-color 0.18s, box-shadow 0.18s;
        }
        .inspiration-card:hover {
          transform: translateY(-1px);
          border-color: var(--semi-color-primary-light-default);
          box-shadow: 0 18px 42px -30px rgba(15, 23, 42, 0.5);
        }
        .inspiration-card img {
          display: block;
          width: 100%;
          height: auto;
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
          .inspiration-masonry { column-gap: 10px; }
          .inspiration-card {
            margin-bottom: 10px;
          }
          .inspiration-detail-actions {
            grid-template-columns: 1fr;
          }
        }
      `}</style>

      <div className='inspiration-shell' ref={shellRef}>
        <Spin spinning={loading}>
          <div
            className='inspiration-masonry'
            style={{ columnCount: masonryColumnCount }}
          >
            {assets.map(renderAssetCard)}
          </div>
          <div ref={sentinelRef} className='inspiration-load-more'>
            {loadingMore && <Spin size='small' />}
          </div>
        </Spin>
      </div>

      <SideSheet
        placement='right'
        visible={detailVisible}
        width={isMobile ? '100%' : 520}
        title={t('作品详情')}
        onCancel={() => setDetailVisible(false)}
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
