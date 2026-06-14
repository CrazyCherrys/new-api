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

const normalizeSessionId = (value) => String(value || '').trim();

const normalizeMode = (mode, canvasModes = []) =>
  canvasModes.includes(mode) ? mode : '';

export const getVisibleCanvasSessionId = ({
  generationMode = '',
  routeSessionId = '',
  selectedCanvasSessionId = null,
  canvasModes = [],
  findCanvasSessionByIdentifier,
}) => {
  const normalizedSelectedId = normalizeSessionId(selectedCanvasSessionId);
  if (normalizedSelectedId) {
    return normalizedSelectedId;
  }
  const normalizedMode = normalizeMode(generationMode, canvasModes);
  const normalizedRouteId = normalizeSessionId(routeSessionId);
  if (!normalizedMode || !normalizedRouteId) {
    return '';
  }
  if (typeof findCanvasSessionByIdentifier !== 'function') {
    return normalizedRouteId;
  }
  const routeSession = findCanvasSessionByIdentifier(
    normalizedRouteId,
    generationMode,
  );
  if (normalizeMode(routeSession?.mode, canvasModes) !== normalizedMode) {
    return '';
  }
  return normalizedRouteId;
};

export const getVisibleCanvasSession = ({
  generationMode = '',
  routeSessionId = '',
  selectedCanvasSessionId = null,
  findCanvasSessionByIdentifier,
  canvasModes = [],
}) => {
  const normalizedSelectedId = normalizeSessionId(selectedCanvasSessionId);
  if (normalizedSelectedId && typeof findCanvasSessionByIdentifier === 'function') {
    return (
      findCanvasSessionByIdentifier(normalizedSelectedId, generationMode) || null
    );
  }
  const visibleSessionId = getVisibleCanvasSessionId({
    generationMode,
    routeSessionId,
    selectedCanvasSessionId,
    canvasModes,
    findCanvasSessionByIdentifier,
  });
  if (!visibleSessionId || typeof findCanvasSessionByIdentifier !== 'function') {
    return null;
  }
  return (
    findCanvasSessionByIdentifier(visibleSessionId, generationMode) || null
  );
};

export const getDisplayedCanvasMessages = ({
  visibleSessionId = '',
  canvasMessagesSessionId = '',
  canvasMessages = [],
}) => {
  return normalizeSessionId(canvasMessagesSessionId) ===
    normalizeSessionId(visibleSessionId)
    ? canvasMessages
    : [];
};
