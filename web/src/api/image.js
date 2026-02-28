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
  return res.data;
};

/**
 * 查询单个图像任务
 * @param {string} taskId - 任务ID
 * @returns {Promise<Object>} 任务详情
 */
export const getImageTask = async (taskId) => {
  const res = await API.get(`/api/images/history/${taskId}`);
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
