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
import {
  Button,
  Empty,
  Form,
  Modal,
  Pagination,
  Spin,
  TabPane,
  Tabs,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconCheckCircleStroked,
  IconClose,
  IconImage,
  IconRefresh,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { API, showError, showSuccess } from '../../../helpers';

const { Paragraph } = Typography;

const PAGE_SIZE = 10;

const SettingsCreativeSpaceReview = () => {
  const { t } = useTranslation();
  const [status, setStatus] = useState('pending');
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [submissions, setSubmissions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [reviewingId, setReviewingId] = useState(null);
  const [rejectTarget, setRejectTarget] = useState(null);
  const [rejectReason, setRejectReason] = useState('');
  const currentListRef = useRef({ status: 'pending', page: 1 });
  const loadSeqRef = useRef(0);

  const statusMeta = useMemo(
    () => ({
      pending: { label: t('待审核'), color: 'orange' },
      approved: { label: t('已通过'), color: 'green' },
      rejected: { label: t('已驳回'), color: 'red' },
    }),
    [t],
  );

  useEffect(() => {
    currentListRef.current = { status, page };
  }, [status, page]);

  const loadSubmissions = useCallback(
    async (nextStatus = status, nextPage = page) => {
      const requestSeq = loadSeqRef.current + 1;
      loadSeqRef.current = requestSeq;
      setLoading(true);
      try {
        const res = await API.get('/api/image-generation/creative-submissions', {
          params: { status: nextStatus, p: nextPage, page_size: PAGE_SIZE },
        });
        if (requestSeq !== loadSeqRef.current) return;
        if (res.data.success) {
          const data = res.data.data || {};
          setSubmissions(data.items || []);
          setTotal(data.total || 0);
        } else {
          showError(res.data.message || t('加载创意空间审核列表失败'));
        }
      } catch (error) {
        if (requestSeq !== loadSeqRef.current) return;
        showError(error.message || t('加载创意空间审核列表失败'));
      } finally {
        if (requestSeq === loadSeqRef.current) {
          setLoading(false);
        }
      }
    },
    [page, status, t],
  );

  useEffect(() => {
    loadSubmissions(status, page);
  }, [loadSubmissions, status, page]);

  const reviewSubmission = async (item, nextStatus, reason = '') => {
    setReviewingId(item.id);
    try {
      const res = await API.patch(
        `/api/image-generation/creative-submissions/${item.id}/review`,
        {
          status: nextStatus,
          reject_reason: reason,
        },
      );
      if (res.data.success) {
        showSuccess(nextStatus === 'approved' ? t('已通过审核') : t('已驳回'));
        setRejectTarget(null);
        setRejectReason('');
        const currentList = currentListRef.current;
        await loadSubmissions(currentList.status, currentList.page);
      } else {
        showError(res.data.message || t('审核操作失败'));
      }
    } catch (error) {
      showError(error.message || t('审核操作失败'));
    } finally {
      setReviewingId(null);
    }
  };

  const formatTime = (timestamp) => {
    if (!timestamp) return '-';
    return dayjs(timestamp * 1000).format('YYYY/MM/DD HH:mm');
  };

  const renderSubmission = (item) => {
    const meta = statusMeta[item.status] || statusMeta.pending;
    return (
      <div className='creative-review-item' key={item.id}>
        <div className='creative-review-preview'>
          {item.image_url ? (
            <img src={item.image_url} alt={item.prompt || 'Generated'} />
          ) : (
            <IconImage size='extra-large' />
          )}
        </div>
        <div className='creative-review-main'>
          <div className='creative-review-head'>
            <div className='creative-review-title'>
              {item.display_name || item.model_id || t('未知模型')}
            </div>
            <Tag color={meta.color}>{meta.label}</Tag>
          </div>
          <Paragraph
            copyable={{ content: item.prompt || '' }}
            ellipsis={{ rows: 2 }}
            style={{ margin: 0 }}
          >
            {item.prompt || '-'}
          </Paragraph>
          <div className='creative-review-meta'>
            <span>
              {t('提交用户')}:{' '}
              {item.user_display_name || item.username || item.user_id}
            </span>
            <span>
              {t('提交时间')}: {formatTime(item.submitted_time)}
            </span>
            {item.reviewed_time > 0 && (
              <span>
                {t('审核时间')}: {formatTime(item.reviewed_time)}
              </span>
            )}
          </div>
          {item.reject_reason && (
            <div className='creative-review-reason'>
              {t('驳回原因')}: {item.reject_reason}
            </div>
          )}
        </div>
        <div className='creative-review-actions'>
          {item.status !== 'approved' && (
            <Button
              type='primary'
              icon={<IconCheckCircleStroked />}
              loading={reviewingId === item.id}
              onClick={() => reviewSubmission(item, 'approved')}
            >
              {t('通过')}
            </Button>
          )}
          {item.status !== 'rejected' && (
            <Button
              type='danger'
              icon={<IconClose />}
              loading={reviewingId === item.id}
              onClick={() => {
                setRejectTarget(item);
                setRejectReason('');
              }}
            >
              {t('驳回')}
            </Button>
          )}
        </div>
      </div>
    );
  };

  return (
    <Form.Section
      text={t('创意空间审核')}
      extraText={t('审核用户提交的图片资产，通过后会公开显示在创意空间')}
    >
      <style>{`
        .creative-review-toolbar {
          display: flex;
          justify-content: space-between;
          align-items: center;
          gap: 12px;
          margin-bottom: 12px;
        }
        .creative-review-list {
          display: flex;
          flex-direction: column;
          gap: 10px;
        }
        .creative-review-item {
          display: grid;
          grid-template-columns: 112px minmax(0, 1fr) auto;
          gap: 14px;
          align-items: stretch;
          padding: 12px;
          border: 1px solid var(--semi-color-border);
          border-radius: 8px;
          background: var(--semi-color-bg-1);
        }
        .creative-review-preview {
          width: 112px;
          aspect-ratio: 1 / 1;
          border-radius: 8px;
          overflow: hidden;
          background: var(--semi-color-fill-0);
          display: flex;
          align-items: center;
          justify-content: center;
          color: var(--semi-color-text-2);
        }
        .creative-review-preview img {
          width: 100%;
          height: 100%;
          object-fit: cover;
          display: block;
        }
        .creative-review-main {
          min-width: 0;
          display: flex;
          flex-direction: column;
          gap: 8px;
        }
        .creative-review-head {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 8px;
        }
        .creative-review-title {
          min-width: 0;
          font-size: 14px;
          font-weight: 650;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
        .creative-review-meta {
          display: flex;
          flex-wrap: wrap;
          gap: 8px 14px;
          color: var(--semi-color-text-2);
          font-size: 12px;
        }
        .creative-review-reason {
          color: var(--semi-color-danger);
          font-size: 12px;
          overflow-wrap: anywhere;
        }
        .creative-review-actions {
          display: flex;
          flex-direction: column;
          gap: 8px;
          justify-content: center;
          min-width: 86px;
        }
        .creative-review-pagination {
          display: flex;
          justify-content: center;
          padding: 14px 0 0;
        }
        @media (max-width: 720px) {
          .creative-review-toolbar {
            align-items: stretch;
            flex-direction: column;
          }
          .creative-review-item {
            grid-template-columns: 86px minmax(0, 1fr);
          }
          .creative-review-preview {
            width: 86px;
          }
          .creative-review-actions {
            grid-column: 1 / -1;
            flex-direction: row;
            justify-content: flex-start;
          }
        }
      `}</style>
      <div className='creative-review-toolbar'>
        <Tabs
          type='button'
          activeKey={status}
          onChange={(key) => {
            setStatus(key);
            setPage(1);
          }}
        >
          {Object.entries(statusMeta).map(([key, meta]) => (
            <TabPane key={key} itemKey={key} tab={meta.label} />
          ))}
        </Tabs>
        <Button
          icon={<IconRefresh />}
          onClick={() => loadSubmissions(status, page)}
        >
          {t('刷新')}
        </Button>
      </div>
      <Spin spinning={loading}>
        {submissions.length > 0 ? (
          <>
            <div className='creative-review-list'>
              {submissions.map(renderSubmission)}
            </div>
            {total > PAGE_SIZE && (
              <div className='creative-review-pagination'>
                <Pagination
                  total={total}
                  currentPage={page}
                  pageSize={PAGE_SIZE}
                  onPageChange={setPage}
                />
              </div>
            )}
          </>
        ) : (
          <Empty
            image={<IconImage size='extra-large' />}
            title={t('暂无审核作品')}
          />
        )}
      </Spin>
      <Modal
        visible={Boolean(rejectTarget)}
        title={t('驳回投稿')}
        okText={t('确认驳回')}
        cancelText={t('取消')}
        onCancel={() => {
          setRejectTarget(null);
          setRejectReason('');
        }}
        onOk={() => reviewSubmission(rejectTarget, 'rejected', rejectReason)}
        confirmLoading={Boolean(
          rejectTarget && reviewingId === rejectTarget.id,
        )}
      >
        <TextArea
          value={rejectReason}
          onChange={setRejectReason}
          autosize
          placeholder={t('填写驳回原因，可选')}
        />
      </Modal>
    </Form.Section>
  );
};

export default SettingsCreativeSpaceReview;
