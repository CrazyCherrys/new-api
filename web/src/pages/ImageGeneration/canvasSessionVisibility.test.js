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
  getDisplayedCanvasMessages,
  getVisibleCanvasSession,
  getVisibleCanvasSessionId,
} from './canvasSessionVisibility';

const CANVAS_MODES = ['chat', 'image', 'video'];

describe('canvasSessionVisibility', () => {
  test('uses the selected session id when it matches the route session id', () => {
    expect(
      getVisibleCanvasSessionId({
        generationMode: 'chat',
        routeSessionId: 'chat-1',
        selectedCanvasSessionId: 'chat-1',
        canvasModes: CANVAS_MODES,
      }),
    ).toBe('chat-1');
  });

  test('prefers the selected session id when the route still points to an old session', () => {
    expect(
      getVisibleCanvasSessionId({
        generationMode: 'chat',
        routeSessionId: 'image-1',
        selectedCanvasSessionId: 'chat-2',
        canvasModes: CANVAS_MODES,
      }),
    ).toBe('chat-2');
  });

  test('falls back to the route session only when the current mode has no selected session', () => {
    const findCanvasSessionByIdentifier = (sessionId) =>
      sessionId === 'chat-3' ? { id: 33, public_id: 'chat-3', mode: 'chat' } : null;

    expect(
      getVisibleCanvasSessionId({
        generationMode: 'chat',
        routeSessionId: 'chat-3',
        selectedCanvasSessionId: null,
        canvasModes: CANVAS_MODES,
        findCanvasSessionByIdentifier,
      }),
    ).toBe('chat-3');

    expect(
      getVisibleCanvasSessionId({
        generationMode: 'chat',
        routeSessionId: 'image-1',
        selectedCanvasSessionId: null,
        canvasModes: CANVAS_MODES,
        findCanvasSessionByIdentifier,
      }),
    ).toBe('');
  });

  test('old image route plus new chat session keeps optimistic messages visible', () => {
    const optimisticMessages = [
      { id: 'req-1-user', role: 'user', prompt: 'hello' },
      { id: 'req-1-assistant', role: 'assistant', prompt: '', status: 'generating' },
    ];
    const findCanvasSessionByIdentifier = (sessionId) =>
      sessionId === 'chat-4' ? { id: 44, public_id: 'chat-4', mode: 'chat' } : null;

    const visibleSessionId = getVisibleCanvasSessionId({
      generationMode: 'chat',
      routeSessionId: 'image-9',
      selectedCanvasSessionId: 'chat-4',
      canvasModes: CANVAS_MODES,
      findCanvasSessionByIdentifier,
    });

    expect(visibleSessionId).toBe('chat-4');
    expect(
      getVisibleCanvasSession({
        generationMode: 'chat',
        routeSessionId: 'image-9',
        selectedCanvasSessionId: 'chat-4',
        findCanvasSessionByIdentifier,
        canvasModes: CANVAS_MODES,
      }),
    ).toEqual({ id: 44, public_id: 'chat-4', mode: 'chat' });
    expect(
      getDisplayedCanvasMessages({
        visibleSessionId,
        canvasMessagesSessionId: 'chat-4',
        canvasMessages: optimisticMessages,
      }),
    ).toEqual(optimisticMessages);
  });
});
