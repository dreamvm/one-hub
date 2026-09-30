import { describe, it, expect, vi } from 'vitest';
import { loader } from '@monaco-editor/react';
import * as monaco from 'monaco-editor';
import EditorWorker from 'monaco-editor/editor/editor.worker?worker';
import JsonWorker from 'monaco-editor/languages/features/json/json.worker?worker';
import '../src/ui-component/JsonEditorRuntime';

// Keep the real loader: an unconfigured loader would append a CDN script and never resolve here.
vi.mock('monaco-editor', () => ({ editor: { create: vi.fn() } }));
vi.mock('monaco-editor/editor/editor.worker?worker', () => ({ default: class EditorWorker {} }));
vi.mock('monaco-editor/languages/features/json/json.worker?worker', () => ({ default: class JsonWorker {} }));

describe('bundled Monaco runtime', () => {
  it('initializes the locked local instance without a remote script', async () => {
    const scripts = document.scripts.length;
    expect(await loader.init()).toBe(monaco);
    expect(await loader.init()).toBe(monaco);
    expect(document.scripts.length).toBe(scripts);
  });

  it('provides separate local JSON workers and the core editor worker', () => {
    const json = self.MonacoEnvironment.getWorker('', 'json');
    expect(json).toBeInstanceOf(JsonWorker);
    expect(self.MonacoEnvironment.getWorker('', 'json')).not.toBe(json);
    expect(self.MonacoEnvironment.getWorker('', 'editorWorkerService')).toBeInstanceOf(EditorWorker);
  });
});
