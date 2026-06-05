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

import { describe, expect, test } from 'bun:test';
import {
  CHAT_CAPABILITY_FILE_UPLOAD,
  CHAT_CAPABILITY_IMAGE_UPLOAD,
  CHAT_CAPABILITY_WEB_SEARCH,
  extractCanvasChatAttachments,
  getCanvasChatUploadVisibility,
  getCanvasChatWebSearchVisibility,
  normalizeCanvasChatCapabilities,
} from './canvasChat';

describe('canvas chat helpers', () => {
  test('normalizes chat capabilities', () => {
    expect(
      normalizeCanvasChatCapabilities(
        '[" image_upload ","file_upload","web_search","image_upload","unknown"]',
      ),
    ).toEqual(['image_upload', 'file_upload', 'web_search']);
  });

  test('reports upload visibility for image-only models', () => {
    expect(
      getCanvasChatUploadVisibility({
        chat_capabilities: [CHAT_CAPABILITY_IMAGE_UPLOAD],
      }),
    ).toEqual({
      showImageUpload: true,
      showFileUpload: false,
    });
  });

  test('reports upload visibility for file-only models', () => {
    expect(
      getCanvasChatUploadVisibility({
        chat_capabilities: [CHAT_CAPABILITY_FILE_UPLOAD],
      }),
    ).toEqual({
      showImageUpload: false,
      showFileUpload: true,
    });
  });

  test('reports upload visibility for models supporting both upload types', () => {
    expect(
      getCanvasChatUploadVisibility({
        chat_capabilities: [
          CHAT_CAPABILITY_IMAGE_UPLOAD,
          CHAT_CAPABILITY_FILE_UPLOAD,
        ],
      }),
    ).toEqual({
      showImageUpload: true,
      showFileUpload: true,
    });
  });

  test('reports upload visibility for models without upload capabilities', () => {
    expect(
      getCanvasChatUploadVisibility({
        chat_capabilities: [],
      }),
    ).toEqual({
      showImageUpload: false,
      showFileUpload: false,
    });
  });

  test('reports web search visibility for supported models', () => {
    expect(
      getCanvasChatWebSearchVisibility({
        chat_capabilities: [CHAT_CAPABILITY_WEB_SEARCH],
      }),
    ).toBe(true);
  });

  test('reports web search visibility for unsupported models', () => {
    expect(
      getCanvasChatWebSearchVisibility({
        chat_capabilities: [CHAT_CAPABILITY_IMAGE_UPLOAD],
      }),
    ).toBe(false);
  });

  test('extracts persisted chat attachments from message metadata', () => {
    expect(
      extractCanvasChatAttachments({
        metadata: JSON.stringify({
          attachments: [
            {
              kind: 'image',
              name: 'reference.png',
              mime_type: 'image/png',
              data: 'data:image/png;base64,Zm9v',
            },
            {
              kind: 'file',
              name: 'notes.txt',
              mime_type: 'text/plain',
              data: 'data:text/plain;base64,YmFy',
            },
          ],
        }),
      }),
    ).toEqual([
      {
        kind: 'image',
        name: 'reference.png',
        mime_type: 'image/png',
        data: 'data:image/png;base64,Zm9v',
      },
      {
        kind: 'file',
        name: 'notes.txt',
        mime_type: 'text/plain',
        data: 'data:text/plain;base64,YmFy',
      },
    ]);
  });
});
