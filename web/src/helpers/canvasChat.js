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

export const CHAT_CAPABILITY_IMAGE_UPLOAD = 'image_upload';
export const CHAT_CAPABILITY_FILE_UPLOAD = 'file_upload';
export const CHAT_CAPABILITY_WEB_SEARCH = 'web_search';

const SUPPORTED_CHAT_CAPABILITIES = new Set([
  CHAT_CAPABILITY_IMAGE_UPLOAD,
  CHAT_CAPABILITY_FILE_UPLOAD,
  CHAT_CAPABILITY_WEB_SEARCH,
]);

const normalizeCanvasChatCapability = (value) =>
  String(value || '')
    .trim()
    .toLowerCase();

export const normalizeCanvasChatCapabilities = (raw) => {
  const source = Array.isArray(raw)
    ? raw
    : (() => {
        if (typeof raw !== 'string' || raw.trim() === '') {
          return [];
        }
        try {
          const parsed = JSON.parse(raw);
          return Array.isArray(parsed) ? parsed : [];
        } catch (error) {
          return [];
        }
      })();

  const normalized = [];
  const seen = new Set();
  source.forEach((item) => {
    const capability = normalizeCanvasChatCapability(item);
    if (!capability || !SUPPORTED_CHAT_CAPABILITIES.has(capability)) {
      return;
    }
    if (seen.has(capability)) {
      return;
    }
    seen.add(capability);
    normalized.push(capability);
  });
  return normalized;
};

export const modelSupportsChatCapability = (model, capability) =>
  normalizeCanvasChatCapabilities(model?.chat_capabilities).includes(
    normalizeCanvasChatCapability(capability),
  );

export const getCanvasChatUploadVisibility = (model) => ({
  showImageUpload: modelSupportsChatCapability(
    model,
    CHAT_CAPABILITY_IMAGE_UPLOAD,
  ),
  showFileUpload: modelSupportsChatCapability(
    model,
    CHAT_CAPABILITY_FILE_UPLOAD,
  ),
});

export const getCanvasChatWebSearchVisibility = (model) =>
  modelSupportsChatCapability(model, CHAT_CAPABILITY_WEB_SEARCH);

const normalizeCanvasChatAttachmentKind = (value) =>
  String(value || '')
    .trim()
    .toLowerCase();

export const normalizeCanvasChatAttachment = (attachment) => {
  const kind = normalizeCanvasChatAttachmentKind(attachment?.kind);
  const name = String(attachment?.name || '').trim();
  const mimeType = String(attachment?.mime_type || '').trim();
  const data = String(attachment?.data || '').trim();
  if (!kind || !name || !mimeType || !data) {
    return null;
  }
  if (kind !== 'image' && kind !== 'file') {
    return null;
  }
  return {
    kind,
    name,
    mime_type: mimeType,
    data,
  };
};

export const extractCanvasChatAttachments = (message) => {
  const metadata = message?.metadata;
  if (typeof metadata !== 'string' || metadata.trim() === '') {
    return [];
  }
  try {
    const parsed = JSON.parse(metadata);
    if (!Array.isArray(parsed?.attachments)) {
      return [];
    }
    return parsed.attachments
      .map((attachment) => normalizeCanvasChatAttachment(attachment))
      .filter(Boolean);
  } catch (error) {
    return [];
  }
};
