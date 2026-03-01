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

import { API } from '../helpers/api';

/**
 * 归一化任务数据，处理后端字段差异
 * @param {Object} task - 原始任务数据
 * @returns {Object} 归一化后的任务数据
 */
const normalizeTask = (task) => {
  if (!task) return null;

  return {
    ...task,
    // 确保 id 字段存在（后端现在返回 id 而非 task_id）
    id: task.id || task.task_id,
    // 保持 image_urls 数组格式
    image_urls: task.image_urls || [],
    // 为兼容性添加 result_url（首图）
    result_url: task.image_urls && task.image_urls.length > 0 ? task.image_urls[0] : null,
  };
};

/**
 * 归一化任务列表
 */
const normalizeTasks = (tasks) => {
  if (!Array.isArray(tasks)) return [];
  return tasks.map(normalizeTask);
};

/**
 * 创建图像生成任务
 * @param {Object} payload - 任务参数
 * @param {string} payload.model - 模型名称
 * @param {string} payload.prompt - 提示词
 * @param {string} [payload.resolution] - 分辨率
 * @param {string} [payload.aspect_ratio] - 宽高比
 * @param {string} [payload.reference_image] - 参考图片URL
 * @param {number} [payload.count] - 生成数量
 * @returns {Promise<{task_id: string, status: string}>}
 */
export const createImageTask = async (payload) => {
  const res = await API.post('/api/images/generate', payload);
  if (res.data.success && res.data.data) {
    res.data.data = normalizeTask(res.data.data);
  }
  return res.data;
};

/**
 * 查询单个图像任务
 * @param {string} taskId - 任务ID
 * @returns {Promise<Object>} 任务详情
 */
export const getImageTask = async (taskId) => {
  const res = await API.get(`/api/images/history/${taskId}`);
  if (res.data.success && res.data.data) {
    res.data.data = normalizeTask(res.data.data);
  }
  return res.data;
};

/**
 * 查询图像任务列表
 * @param {Object} params - 查询参数
 * @param {number} [params.page] - 页码
 * @param {number} [params.page_size] - 每页数量
 * @param {string} [params.status] - 任务状态
 * @param {string} [params.model] - 模型名称
 * @param {number} [params.start_time] - 开始时间戳
 * @param {number} [params.end_time] - 结束时间戳
 * @returns {Promise<{data: Array, total: number, page: number, page_size: number}>}
 */
export const listImageTasks = async (params = {}) => {
  const res = await API.get('/api/images/history', { params });
  if (res.data.success && res.data.data) {
    // 后端返回 data.data，归一化为 data.tasks
    res.data.data = {
      tasks: normalizeTasks(res.data.data.data || res.data.data),
      total: res.data.data.total || 0,
      page: res.data.data.page || params.page || 1,
      page_size: res.data.data.page_size || params.page_size || 10,
    };
  }
  return res.data;
};

/**
 * 删除图像任务
 * @param {string} taskId - 任务ID
 * @returns {Promise<Object>}
 */
export const deleteImageTask = async (taskId) => {
  const res = await API.delete(`/api/images/history/${taskId}`);
  return res.data;
};
