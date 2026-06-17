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
  getChatRouteSyncAction,
  getRouteSelectionSyncAction,
} from './canvasSessionRouting';

const CANVAS_MODES = ['chat', 'image', 'video'];

const getCanvasSessionIdentifier = (session) => String(session?.public_id || session?.id || '');

describe('canvasSessionRouting', () => {
  test('route selection chooses the routed chat session when local state still points to an older chat', () => {
    const findCanvasSessionByIdentifier = (sessionId) =>
      sessionId === 'chat-2' ? { id: 22, public_id: 'chat-2', mode: 'chat' } : null;

    expect(
      getRouteSelectionSyncAction({
        routeSessionId: 'chat-2',
        generationMode: 'chat',
        selectedSessionIdsByMode: {
          chat: 'chat-1',
          image: 'image-1',
          video: null,
        },
        findCanvasSessionByIdentifier,
        getCanvasSessionIdentifier,
        canvasModes: CANVAS_MODES,
        defaultMode: 'image',
      }),
    ).toMatchObject({
      type: 'select-route-session',
      sessionIdentifier: 'chat-2',
      sessionMode: 'chat',
    });
  });

  test('route selection is a noop when route and selected state already match', () => {
    const findCanvasSessionByIdentifier = (sessionId) =>
      sessionId === 'chat-2' ? { id: 22, public_id: 'chat-2', mode: 'chat' } : null;

    expect(
      getRouteSelectionSyncAction({
        routeSessionId: 'chat-2',
        generationMode: 'chat',
        selectedSessionIdsByMode: {
          chat: 'chat-2',
          image: 'image-1',
          video: null,
        },
        findCanvasSessionByIdentifier,
        getCanvasSessionIdentifier,
        canvasModes: CANVAS_MODES,
        defaultMode: 'image',
      }),
    ).toMatchObject({
      type: 'noop',
      sessionIdentifier: 'chat-2',
      sessionMode: 'chat',
    });
  });

  test('route selection stays noop when the routed session is already selected for its own mode even if generation mode changed locally', () => {
    const findCanvasSessionByIdentifier = (sessionId) =>
      sessionId === 'chat-2' ? { id: 22, public_id: 'chat-2', mode: 'chat' } : null;

    expect(
      getRouteSelectionSyncAction({
        routeSessionId: 'chat-2',
        selectedSessionIdsByMode: {
          chat: 'chat-2',
          image: null,
          video: null,
        },
        findCanvasSessionByIdentifier,
        getCanvasSessionIdentifier,
        canvasModes: CANVAS_MODES,
        defaultMode: 'image',
      }),
    ).toMatchObject({
      type: 'noop',
      sessionIdentifier: 'chat-2',
      sessionMode: 'chat',
    });
  });

  test('route selection clears all mode selections when route is blank', () => {
    expect(
      getRouteSelectionSyncAction({
        routeSessionId: '',
        previousRouteSessionId: 'chat-1',
        generationMode: 'chat',
        selectedSessionIdsByMode: {
          chat: 'chat-1',
          image: 'image-1',
          video: 'video-1',
        },
        findCanvasSessionByIdentifier: () => null,
        getCanvasSessionIdentifier,
        canvasModes: CANVAS_MODES,
        defaultMode: 'image',
      }),
    ).toEqual({
      type: 'clear-selection',
    });
  });

  test('route selection stays noop when local state updates while route is still initially blank', () => {
    expect(
      getRouteSelectionSyncAction({
        routeSessionId: '',
        previousRouteSessionId: '',
        selectedSessionIdsByMode: {
          chat: 'chat-1',
          image: null,
          video: null,
        },
        findCanvasSessionByIdentifier: () => null,
        getCanvasSessionIdentifier,
        canvasModes: CANVAS_MODES,
        defaultMode: 'image',
      }),
    ).toEqual({
      type: 'noop',
    });
  });

  test('route selection requests remote lookup when the routed session is not loaded locally', () => {
    expect(
      getRouteSelectionSyncAction({
        routeSessionId: 'chat-99',
        generationMode: 'chat',
        selectedSessionIdsByMode: {
          chat: 'chat-1',
          image: 'image-1',
          video: null,
        },
        findCanvasSessionByIdentifier: () => null,
        getCanvasSessionIdentifier,
        canvasModes: CANVAS_MODES,
        defaultMode: 'image',
      }),
    ).toEqual({
      type: 'lookup-route-session',
      routeSessionId: 'chat-99',
    });
  });

  test('chat route sync prefers the selected chat session when route still points to an old image session', () => {
    expect(
      getChatRouteSyncAction({
        generationMode: 'chat',
        chatMode: 'chat',
        routeSessionId: 'image-3',
        selectedChatSessionId: 'chat-4',
        findCanvasSessionByIdentifier: () => null,
      }),
    ).toEqual({
      type: 'sync-route-to-selected-chat',
      sessionId: 'chat-4',
    });
  });

  test('chat route sync clears the route when blank chat is active but route still points to a non-chat session', () => {
    const findCanvasSessionByIdentifier = (sessionId) =>
      sessionId === 'image-3' ? { id: 33, public_id: 'image-3', mode: 'image' } : null;

    expect(
      getChatRouteSyncAction({
        generationMode: 'chat',
        chatMode: 'chat',
        routeSessionId: 'image-3',
        selectedChatSessionId: '',
        findCanvasSessionByIdentifier,
      }),
    ).toEqual({
      type: 'sync-route-to-blank',
    });
  });
});
