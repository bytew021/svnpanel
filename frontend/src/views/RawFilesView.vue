<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { onBeforeRouteLeave } from "vue-router";
import * as monaco from "monaco-editor/editor/editor.api.js";
import "monaco-editor/editor/contrib/bracketMatching/browser/bracketMatching.js";
import "monaco-editor/editor/contrib/comment/browser/comment.js";
import "monaco-editor/editor/contrib/contextmenu/browser/contextmenu.js";
import "monaco-editor/editor/contrib/find/browser/findController.js";
import "monaco-editor/editor/contrib/folding/browser/folding.js";
import "monaco-editor/editor/contrib/linesOperations/browser/linesOperations.js";
import "monaco-editor/editor/contrib/wordHighlighter/browser/wordHighlighter.js";
import EditorWorker from "monaco-editor/editor/editor.worker.js?worker";
import { errorMessage, getRawFiles, updateRawFile } from "@/api/client";
import type { RawFileName, Revisions } from "@/types";
import { confirmAction, failure, success } from "@/utils/notify";

interface EditableFile {
  content: string;
  original: string;
  revision: string;
}

self.MonacoEnvironment = {
  getWorker: () => new EditorWorker(),
};

const languageId = "svnconfig";
if (!monaco.languages.getLanguages().some((language) => language.id === languageId)) {
  monaco.languages.register({ id: languageId });
  monaco.languages.setLanguageConfiguration(languageId, {
    comments: { lineComment: "#" },
    brackets: [
      ["[", "]"],
      ["{", "}"],
      ["(", ")"],
    ],
    autoClosingPairs: [
      { open: "[", close: "]" },
      { open: "{", close: "}" },
      { open: "(", close: ")" },
      { open: '"', close: '"' },
    ],
  });
  monaco.languages.setMonarchTokensProvider(languageId, {
    tokenizer: {
      root: [
        [/^\s*\/\/.*/, "comment"],
        [/^\s*#disabled#/, "invalid"],
        [/^\s*#.*$/, "comment"],
        [/^\s*\[[^\]\r\n]+\]/, "type"],
        [/^\s*@[A-Za-z0-9._-]+/, "tag"],
        [/^\s*(?:\*|\$authenticated|\$anonymous)/, "constant"],
        [
          /^(\s*)([A-Za-z0-9._@*$-]+)(\s*)(=)(\s*)(rw|r)(\s*)$/,
          ["white", "variable", "white", "operator", "white", "keyword", "white"],
        ],
        [/^\s*[A-Za-z0-9._-]+(?=\s*=)/, "variable"],
        [/=/, "operator"],
      ],
    },
  });
}
monaco.editor.defineTheme("svnpanel-raw", {
  base: "vs-dark",
  inherit: true,
  rules: [
    { token: "comment", foreground: "7D8CA3", fontStyle: "italic" },
    { token: "type", foreground: "7DD3FC", fontStyle: "bold" },
    { token: "variable", foreground: "C4B5FD" },
    { token: "tag", foreground: "F9A8D4" },
    { token: "constant", foreground: "FBBF24" },
    { token: "keyword", foreground: "86EFAC", fontStyle: "bold" },
    { token: "operator", foreground: "94A3B8" },
    { token: "invalid", foreground: "FCA5A5", fontStyle: "bold" },
  ],
  colors: {
    "editor.background": "#0B1220",
    "editor.foreground": "#DCE6F5",
    "editorLineNumber.foreground": "#52647A",
    "editorLineNumber.activeForeground": "#93C5FD",
    "editorCursor.foreground": "#60A5FA",
    "editor.selectionBackground": "#1D4ED866",
    "editor.lineHighlightBackground": "#111D31",
    "editorIndentGuide.background1": "#1E293B",
    "editorIndentGuide.activeBackground1": "#475569",
    "editorGutter.background": "#0B1220",
  },
});

const fileNames: RawFileName[] = ["passwd", "authz"];
const fileLabels: Record<RawFileName, string> = {
  passwd: "passwd",
  authz: "authz",
};
const activeFile = ref<RawFileName>("passwd");
const files = reactive<Record<RawFileName, EditableFile>>({
  passwd: { content: "", original: "", revision: "" },
  authz: { content: "", original: "", revision: "" },
});
const revisions = ref<Revisions>({ passwd: "", authz: "" });
const loading = ref(false);
const saving = ref(false);
const editorHost = ref<HTMLElement>();
const cursor = reactive({ line: 1, column: 1 });
const models = new Map<RawFileName, monaco.editor.ITextModel>();
const viewStates = new Map<RawFileName, monaco.editor.ICodeEditorViewState | null>();
let editor: monaco.editor.IStandaloneCodeEditor | undefined;
let syncing = false;

const dirtyFiles = computed(() =>
  Object.fromEntries(fileNames.map((name) => [name, files[name].content !== files[name].original])) as Record<
    RawFileName,
    boolean
  >,
);
const activeDirty = computed(() => dirtyFiles.value[activeFile.value]);
const hasDirty = computed(() => fileNames.some((name) => dirtyFiles.value[name]));

function shortRevision(revision: string) {
  return revision ? revision.slice(0, 10) : "—";
}

function ensureModel(name: RawFileName) {
  let model = models.get(name);
  if (!model) {
    model = monaco.editor.createModel(
      files[name].content,
      languageId,
      monaco.Uri.parse(`inmemory://svnpanel/${name}`),
    );
    model.onDidChangeContent(() => {
      if (!syncing) files[name].content = model?.getValue() ?? "";
    });
    models.set(name, model);
  } else {
    syncing = true;
    model.setValue(files[name].content);
    syncing = false;
  }
  return model;
}

function ensureEditor() {
  if (editor || !editorHost.value) return;
  editor = monaco.editor.create(editorHost.value, {
    model: ensureModel(activeFile.value),
    theme: "svnpanel-raw",
    language: languageId,
    automaticLayout: true,
    fontFamily: '"Cascadia Code", "JetBrains Mono", Consolas, monospace',
    fontSize: 13,
    lineHeight: 22,
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    renderWhitespace: "selection",
    folding: true,
    tabSize: 4,
    insertSpaces: true,
    fixedOverflowWidgets: true,
    overviewRulerBorder: false,
    stickyScroll: { enabled: false },
    padding: { top: 14, bottom: 14 },
  });
  editor.onDidChangeCursorPosition(({ position }) => {
    cursor.line = position.lineNumber;
    cursor.column = position.column;
  });
  editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
    void save();
  });
}

async function load(): Promise<boolean> {
  loading.value = true;
  try {
    const result = await getRawFiles();
    revisions.value = result.revisions;
    files.passwd = {
      content: result.passwd.content,
      original: result.passwd.content,
      revision: result.passwd.revision,
    };
    files.authz = {
      content: result.authz.content,
      original: result.authz.content,
      revision: result.authz.revision,
    };
    await nextTick();
    ensureModel("passwd");
    ensureModel("authz");
    ensureEditor();
    editor?.setModel(models.get(activeFile.value) ?? null);
    return true;
  } catch (error) {
    failure(errorMessage(error));
    return false;
  } finally {
    loading.value = false;
  }
}

function switchFile(name: RawFileName) {
  if (name === activeFile.value) return;
  if (editor) viewStates.set(activeFile.value, editor.saveViewState());
  const model = ensureModel(name);
  activeFile.value = name;
  if (!editor) return;
  editor.setModel(model);
  const state = viewStates.get(name);
  if (state) editor.restoreViewState(state);
  editor.focus();
}

async function reload() {
  if (hasDirty.value) {
    const confirmed = await confirmAction(
      "放弃未保存的修改？",
      "重新加载会用服务器上的文件内容覆盖 passwd 和 authz 的草稿。",
      true,
    );
    if (!confirmed) return;
  }
  if (await load()) success("原生文件已重新加载");
}

async function save() {
  const name = activeFile.value;
  if (!dirtyFiles.value[name] || saving.value) return;
  saving.value = true;
  try {
    const result = await updateRawFile(name, revisions.value, files[name].content);
    revisions.value = result.revisions;
    files[name].original = files[name].content;
    files[name].revision = result.revisions[name];
    const other = name === "passwd" ? "authz" : "passwd";
    files[other].revision = result.revisions[other];
    success(`${fileLabels[name]} 已保存`);
  } catch (error) {
    failure(errorMessage(error));
  } finally {
    saving.value = false;
  }
}

function beforeUnload(event: BeforeUnloadEvent) {
  if (!hasDirty.value) return;
  event.preventDefault();
  event.returnValue = "";
}

onBeforeRouteLeave(async () => {
  if (!hasDirty.value) return true;
  return confirmAction("离开原生文件编辑页？", "当前存在未保存的修改，离开后会丢失。", true);
});

onMounted(() => {
  void load();
  window.addEventListener("beforeunload", beforeUnload);
});
onBeforeUnmount(() => {
  window.removeEventListener("beforeunload", beforeUnload);
  editor?.dispose();
  for (const model of models.values()) model.dispose();
  models.clear();
});
</script>

<template>
  <div class="flex h-full min-h-0 flex-col gap-4">
    <section class="card flex min-h-0 flex-1 flex-col overflow-hidden">
      <header class="shrink-0 border-b border-slate-200 px-5 py-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 class="text-base font-bold text-slate-900">原生文件编辑</h1>
            <p class="mt-1 text-xs text-slate-500">
              直接编辑共享 passwd 和 authz。保存前仍会校验语法、revision、备份和审计。
            </p>
          </div>
          <div class="flex items-center gap-2">
            <span v-if="hasDirty" class="badge badge-amber">有未保存修改</span>
            <button class="btn btn-secondary" :disabled="loading || saving" @click="reload">
              重新加载
            </button>
            <button class="btn btn-primary min-w-28" :disabled="!activeDirty || saving" @click="save">
              {{ saving ? "保存中…" : "保存 Ctrl+S" }}
            </button>
          </div>
        </div>
        <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
          <div class="inline-flex rounded-lg bg-slate-100 p-1">
            <button
              v-for="name in fileNames"
              :key="name"
              class="relative rounded-md px-5 py-2 font-mono text-sm font-semibold transition"
              :class="activeFile === name ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-800'"
              @click="switchFile(name)"
            >
              {{ fileLabels[name] }}
              <span v-if="dirtyFiles[name]" class="ml-1 inline-block h-1.5 w-1.5 rounded-full bg-amber-500" />
            </button>
          </div>
          <div class="flex items-center gap-4 text-xs text-slate-500">
            <span>revision <code>{{ shortRevision(files[activeFile].revision) }}</code></span>
            <span>UTF-8</span>
            <span>自定义 SVN 配置高亮</span>
          </div>
        </div>
      </header>
      <div class="relative min-h-0 flex-1">
        <div ref="editorHost" class="h-full w-full" />
        <div v-if="loading" class="absolute inset-0 grid place-items-center bg-slate-950/70 text-sm text-slate-200">
          正在加载原生文件…
        </div>
      </div>
      <footer class="flex shrink-0 items-center justify-between border-t border-slate-800 bg-[#0b1220] px-4 py-2 font-mono text-[11px] text-slate-400">
        <span>{{ activeFile }} · {{ files[activeFile].content.split("\n").length }} lines</span>
        <span>Ln {{ cursor.line }}, Col {{ cursor.column }}</span>
      </footer>
    </section>

    <section class="shrink-0 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs leading-6 text-amber-900">
      原生编辑会绕过用户、分组和权限表单。保存时服务端仍会解析 passwd/authz，但不会自动修复人工输入的结构错误。其他浏览器已修改文件时会返回 409 冲突，请先重新加载。
    </section>
  </div>
</template>
