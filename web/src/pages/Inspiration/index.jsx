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
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { Button, Empty, SideSheet, Spin, Typography } from '@douyinfe/semi-ui';
import {
  IconCopy,
  IconDownload,
  IconImage,
} from '@douyinfe/semi-icons';
import { API, copy, showError, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Paragraph } = Typography;
const PAGE_SIZE = 24;

const Inspiration = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const isMobile = useIsMobile();
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
    } catch (e) {
      return {};
    }
  };

  const selectedParams = useMemo(
    () => parseJsonObject(selectedAsset?.params),
    [selectedAsset?.params],
  );

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

  const scrollToTop = () => {
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const renderAssetCard = (asset) => (
    <button
      type='button'
      key={asset.id}
      className='inspiration-card'
      onClick={() => openDetail(asset)}
    >
      <span className='inspiration-image-wrap'>
        <img src={asset.image_url} alt={asset.prompt || 'Generated'} />
      </span>
      <span className='inspiration-card-body'>
        <span className='inspiration-card-model'>
          {asset.display_name || asset.model_id || t('未知模型')}
        </span>
        <span className='inspiration-card-prompt'>{asset.prompt || '-'}</span>
      </span>
    </button>
  );

  return (
    <div className='inspiration-page'>
      <style>{`
        .inspiration-page {
          min-height: calc(100vh - 60px);
          margin-top: 60px;
          padding: 18px 18px 28px;
          color: var(--semi-color-text-0);
          background: var(--semi-color-bg-0);
        }
        .inspiration-shell {
          width: min(1680px, 100%);
          margin: 0 auto;
          display: flex;
          flex-direction: column;
          gap: 16px;
        }
        .creative-topbar {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 12px;
          min-height: 38px;
        }
        .creative-title-wrap {
          display: flex;
          align-items: baseline;
          gap: 10px;
          min-width: 0;
        }
        .creative-title {
          margin: 0;
          font-size: 20px;
          line-height: 1.3;
          font-weight: 700;
          letter-spacing: 0;
          color: var(--semi-color-text-0);
        }
        .creative-count {
          color: var(--semi-color-text-2);
          font-size: 13px;
          white-space: nowrap;
        }
        .creative-actions {
          display: flex;
          gap: 8px;
          flex-shrink: 0;
        }
        .inspiration-masonry {
          column-count: 6;
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
          border-radius: 8px;
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
        .inspiration-image-wrap {
          display: block;
          width: 100%;
          background: var(--semi-color-fill-0);
        }
        .inspiration-image-wrap img {
          display: block;
          width: 100%;
          height: auto;
        }
        .inspiration-card-body {
          display: flex;
          flex-direction: column;
          gap: 5px;
          padding: 9px 10px 11px;
        }
        .inspiration-card-model {
          font-size: 13px;
          line-height: 1.35;
          font-weight: 600;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
        .inspiration-card-prompt {
          color: var(--semi-color-text-2);
          font-size: 12px;
          line-height: 1.45;
          display: -webkit-box;
          -webkit-line-clamp: 2;
          -webkit-box-orient: vertical;
          overflow: hidden;
          overflow-wrap: anywhere;
        }
        .inspiration-load-more {
          display: flex;
          justify-content: center;
          padding: 8px 0 0;
          min-height: 46px;
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
        .inspiration-detail-actions {
          display: grid;
          grid-template-columns: 1fr 1fr;
          gap: 8px;
        }
        @media (max-width: 1480px) {
          .inspiration-masonry { column-count: 5; }
        }
        @media (max-width: 1280px) {
          .inspiration-masonry { column-count: 4; }
        }
        @media (max-width: 960px) {
          .inspiration-masonry { column-count: 3; }
        }
        @media (max-width: 720px) {
          .inspiration-page {
            padding: 8px 8px 20px;
          }
          .creative-topbar {
            align-items: flex-start;
            flex-direction: column;
          }
          .creative-actions {
            width: 100%;
          }
          .inspiration-masonry {
            column-count: 2;
            column-gap: 10px;
          }
          .inspiration-card {
            margin-bottom: 10px;
          }
        }
        @media (max-width: 420px) {
          .inspiration-masonry { column-count: 1; }
        }
      `}</style>

      <div className='inspiration-shell'>
        <div className='creative-topbar'>
          <div className='creative-title-wrap'>
            <h1 className='creative-title'>{t('灵感')}</h1>
            <span className='creative-count'>
              {t('公开作品')} {total}
            </span>
          </div>
          <div className='creative-actions'>
            <Button onClick={scrollToTop}>{t('回到顶部')}</Button>
          </div>
        </div>
        <Spin spinning={loading}>
          {assets.length > 0 ? (
            <>
              <div className='inspiration-masonry'>
                {assets.map(renderAssetCard)}
              </div>
              <div ref={sentinelRef} className='inspiration-load-more'>
                {loadingMore && <Spin size='small' />}
                {!hasMore && total > 0 && (
                  <span className='creative-count'>{t('已加载全部作品')}</span>
                )}
              </div>
            </>
          ) : (
            <Empty
              image={<IconImage size='extra-large' />}
              title={t('暂无公开作品')}
            >
              <Button
                theme='solid'
                type='primary'
                onClick={() => navigate('/ai-generation')}
              >
                {t('去生成图片')}
              </Button>
            </Empty>
          )}
        </Spin>
      </div>

      <SideSheet
        placement='right'
        visible={detailVisible}
        width={isMobile ? '100%' : 520}
        title={t('灵感详情')}
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
