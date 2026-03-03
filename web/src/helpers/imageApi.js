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

import { API } from './api';

const IMAGE_API_BASE = '/api/v1/image-tasks';

const inFlightPostRequests = new Map();

const genPostKey = (url, payload) => {
  return `${url}:${JSON.stringify(payload)}`;
};

const deduplicatePost = (url, payload, requestFn) => {
  const key = genPostKey(url, payload);

  if (inFlightPostRequests.has(key)) {
    return inFlightPostRequests.get(key);
  }

  const reqPromise = requestFn().finally(() => {
    inFlightPostRequests.delete(key);
  });

  inFlightPostRequests.set(key, reqPromise);
  return reqPromise;
};

export const createImageTask = (payload) => {
  return deduplicatePost(
    `${IMAGE_API_BASE}/generate`,
    payload,
    () => API.post(`${IMAGE_API_BASE}/generate`, payload)
  );
};

export const getImageTask = (taskId) => {
  return API.get(`${IMAGE_API_BASE}/history/${taskId}`);
};

export const listImageTasks = (params = {}) => {
  return API.get(`${IMAGE_API_BASE}/history`, { params });
};

export const deleteImageTask = (taskId) => {
  return API.delete(`${IMAGE_API_BASE}/history/${taskId}`);
};

export const getImageConfig = () => {
  return API.get(`${IMAGE_API_BASE}/config`);
};

export const updateImageConfig = (config) => {
  return API.put(`${IMAGE_API_BASE}/config`, config);
};
