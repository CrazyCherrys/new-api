import { describe, expect, test } from 'bun:test';
import { extractCanvasChatReasoning } from './canvasChatReasoning';

describe('canvas chat reasoning extraction', () => {
  test('uses persisted reasoning_content when present', () => {
    const result = extractCanvasChatReasoning({
      prompt: '最终答复',
      reasoning_content: '第一步\n第二步',
    });

    expect(result).toEqual({
      content: '最终答复',
      reasoningContent: '第一步\n第二步',
      hasReasoning: true,
    });
  });

  test('extracts paired think tags from prompt content', () => {
    const result = extractCanvasChatReasoning({
      prompt: '前言<think>思考过程</think>最终答复',
    });

    expect(result).toEqual({
      content: '前言最终答复',
      reasoningContent: '思考过程',
      hasReasoning: true,
    });
  });

  test('extracts unclosed think content from streaming prompt', () => {
    const result = extractCanvasChatReasoning({
      prompt: '最终答复<think>还在思考',
    });

    expect(result).toEqual({
      content: '最终答复',
      reasoningContent: '还在思考',
      hasReasoning: true,
    });
  });

  test('merges reasoning_content with think-tag content in order', () => {
    const result = extractCanvasChatReasoning({
      prompt: '前言<think>标签思考</think>最终答复<think>流式尾巴',
      reasoning_content: '持久化思考',
    });

    expect(result).toEqual({
      content: '前言最终答复',
      reasoningContent: '持久化思考\n\n---\n\n标签思考\n\n---\n\n流式尾巴',
      hasReasoning: true,
    });
  });

  test('returns plain content when no reasoning exists', () => {
    const result = extractCanvasChatReasoning({
      prompt: '只有正文',
    });

    expect(result).toEqual({
      content: '只有正文',
      reasoningContent: '',
      hasReasoning: false,
    });
  });
});
