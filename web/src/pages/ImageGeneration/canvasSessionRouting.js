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

const normalizeValue = (value) => String(value || '').trim();

export const getRouteSelectionSyncAction = ({
  routeSessionId = '',
  selectedSessionIdsByMode = {},
  findCanvasSessionByIdentifier,
  getCanvasSessionIdentifier,
  canvasModes = [],
  defaultMode = '',
}) => {
  const normalizedRouteId = normalizeValue(routeSessionId);
  if (!normalizedRouteId) {
    return {
      type: 'clear-selection',
    };
  }

  if (typeof findCanvasSessionByIdentifier !== 'function') {
    return {
      type: 'lookup-route-session',
      routeSessionId: normalizedRouteId,
    };
  }

  const session = findCanvasSessionByIdentifier(normalizedRouteId);
  if (!session) {
    return {
      type: 'lookup-route-session',
      routeSessionId: normalizedRouteId,
    };
  }

  const sessionIdentifier =
    typeof getCanvasSessionIdentifier === 'function'
      ? normalizeValue(getCanvasSessionIdentifier(session))
      : normalizedRouteId;
  const sessionMode = canvasModes.includes(session.mode) ? session.mode : defaultMode;
  const normalizedSelectedId = normalizeValue(
    sessionMode ? selectedSessionIdsByMode?.[sessionMode] : '',
  );
  if (sessionMode && normalizedSelectedId === sessionIdentifier) {
    return {
      type: 'noop',
      session,
      sessionIdentifier,
      sessionMode,
    };
  }

  return {
    type: 'select-route-session',
    session,
    sessionIdentifier,
    sessionMode,
  };
};

export const getChatRouteSyncAction = ({
  generationMode = '',
  chatMode = '',
  routeSessionId = '',
  selectedChatSessionId = '',
  findCanvasSessionByIdentifier,
}) => {
  if (generationMode !== chatMode) {
    return {
      type: 'noop',
    };
  }

  const normalizedSelectedId = normalizeValue(selectedChatSessionId);
  const normalizedRouteId = normalizeValue(routeSessionId);

  if (normalizedSelectedId) {
    if (normalizedSelectedId === normalizedRouteId) {
      return {
        type: 'noop',
      };
    }
    return {
      type: 'sync-route-to-selected-chat',
      sessionId: normalizedSelectedId,
    };
  }

  if (!normalizedRouteId || typeof findCanvasSessionByIdentifier !== 'function') {
    return {
      type: 'noop',
    };
  }

  const routeSession = findCanvasSessionByIdentifier(normalizedRouteId);
  if (!routeSession || routeSession.mode === chatMode) {
    return {
      type: 'noop',
    };
  }

  return {
    type: 'sync-route-to-blank',
  };
};
