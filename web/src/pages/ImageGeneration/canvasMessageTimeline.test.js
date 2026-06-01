import { describe, expect, test } from 'bun:test';
import {
  mergeCanvasLatestTimelinePage,
  mergeCanvasMessagesById,
  sortCanvasMessagesByCreated,
} from './canvasMessageTimeline';

const createMessage = (id, createdTime, extra = {}) => ({
  id,
  created_time: createdTime,
  prompt: `message-${id}`,
  ...extra,
});

describe('canvas message timeline helpers', () => {
  test('sorts by created_time then id', () => {
    const messages = [
      createMessage(4, 200),
      createMessage(2, 100),
      createMessage(3, 100),
      createMessage(1, 50),
    ];

    const sorted = messages.slice().sort(sortCanvasMessagesByCreated);

    expect(sorted.map((message) => message.id)).toEqual([1, 2, 3, 4]);
  });

  test('mergeCanvasMessagesById upserts and preserves chronological order', () => {
    const merged = mergeCanvasMessagesById(
      [createMessage(2, 100, { status: 'old' }), createMessage(4, 200)],
      [createMessage(1, 50), createMessage(2, 100, { status: 'new' })],
    );

    expect(merged.map((message) => message.id)).toEqual([1, 2, 4]);
    expect(merged.find((message) => message.id === 2)?.status).toBe('new');
  });

  test('same-session latest page refresh preserves already loaded older history', () => {
    const existingMessages = [
      createMessage(1, 50),
      createMessage(2, 100, { prompt: 'older loaded' }),
      createMessage(3, 150),
      createMessage(4, 200),
    ];
    const latestPageMessages = [
      createMessage(3, 150, { prompt: 'fresh latest a' }),
      createMessage(4, 200, { prompt: 'fresh latest b' }),
      createMessage(5, 250, { prompt: 'newest' }),
    ];

    const result = mergeCanvasLatestTimelinePage(
      existingMessages,
      latestPageMessages,
    );

    expect(result.hasOlderLoadedHistory).toBe(true);
    expect(result.messages.map((message) => message.id)).toEqual([1, 2, 3, 4, 5]);
    expect(result.messages.find((message) => message.id === 3)?.prompt).toBe(
      'fresh latest a',
    );
  });

  test('latest page replaces state when no older history is loaded', () => {
    const result = mergeCanvasLatestTimelinePage(
      [createMessage(3, 150), createMessage(4, 200)],
      [createMessage(4, 200, { prompt: 'fresh' }), createMessage(5, 250)],
    );

    expect(result.hasOlderLoadedHistory).toBe(false);
    expect(result.messages.map((message) => message.id)).toEqual([4, 5]);
    expect(result.messages[0].prompt).toBe('fresh');
  });
});
