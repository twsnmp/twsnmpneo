<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import { CodeJar as CreateCodeJar } from "codejar";

  interface Props {
    element?: HTMLElement;
    id?: string;
    class?: string;
    style?: string;
    addClosing?: boolean;
    catchTab?: boolean;
    history?: boolean;
    indentOn?: RegExp;
    preserveIdent?: boolean;
    spellcheck?: boolean;
    tab?: string;
    highlight?: (code: string, syntax?: string) => string;
    syntax?: string;
    value?: string;
  }

  let {
    element = $bindable(undefined),
    id = undefined,
    class: className = "",
    style = "",
    addClosing = false,
    catchTab = false,
    history = true,
    indentOn = /{$/,
    preserveIdent = true,
    spellcheck = false,
    tab = "\t",
    highlight = undefined,
    syntax = undefined,
    value = $bindable(""),
  }: Props = $props();

  let editorElement: HTMLElement;
  let jar: any;
  let ignoreUpdate = false;

  function wrapHighlight(hl: any, syn: any) {
    return hl
      ? (el: HTMLElement) => {
          el.innerHTML = hl(el.textContent ?? "", syn);
        }
      : () => {};
  }

  function handleKeyDownCapture(e: KeyboardEvent) {
    if (e.isComposing || e.keyCode === 229) {
      e.stopPropagation();
    }
  }

  function initJar(hl: any, syn: any, val: string, opts: object) {
    destroyJar();
    if (!editorElement) return;
    editorElement.addEventListener("keydown", handleKeyDownCapture, {
      capture: true,
    });
    jar = CreateCodeJar(editorElement, wrapHighlight(hl, syn), opts);
    jar.updateCode(val);
    jar.onUpdate((code: string) => {
      ignoreUpdate = true;
      value = code;
    });
    if (element === undefined) {
      element = editorElement;
    }
  }

  function destroyJar() {
    if (editorElement) {
      editorElement.removeEventListener("keydown", handleKeyDownCapture, {
        capture: true,
      });
    }
    if (jar) {
      jar.destroy();
      jar = undefined;
    }
  }

  $effect(() => {
    if (editorElement) {
      const hl = highlight;
      const syn = syntax;
      const val = untrack(() => value);
      const opts = {
        addClosing,
        catchTab,
        history,
        indentOn,
        preserveIdent,
        spellcheck,
        tab,
      };
      untrack(() => {
        initJar(hl, syn, val, opts);
      });
    }
  });

  $effect(() => {
    if (jar && !ignoreUpdate) {
      if (value !== jar.toString()) {
        jar.updateCode(value);
      }
    }
    ignoreUpdate = false;
  });

  $effect(() => {
    if (jar) {
      jar.updateOptions({
        addClosing,
        catchTab,
        history,
        indentOn,
        preserveIdent,
        spellcheck,
        tab,
      });
    }
  });

  onDestroy(() => {
    destroyJar();
  });
</script>

<pre
  bind:this={editorElement}
  {id}
  class="{syntax ? `language-${syntax}` : ''} {className}"
  {style}
><code class={syntax ? `language-${syntax}` : ""}>{#if highlight}{@html highlight(value, syntax)}{:else}{value}{/if}</code></pre>

<style>
  pre[class*="language-"] {
    background-color: #f8fafc;
    color: #0f172a;
    border: 1px solid #cbd5e1;
    border-radius: 0.75rem;
    padding: 0.5rem 0.75rem;
    font-size: 0.75rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
      monospace;
    text-shadow: none;
    overflow: auto;
    min-height: 4rem;
    white-space: pre-wrap;
    word-break: break-all;
    outline: none;
  }

  pre[class*="language-"]:focus {
    border-color: #06b6d4;
    box-shadow: 0 0 0 2px rgba(6, 182, 212, 0.3);
  }

  :global(.dark) pre[class*="language-"] {
    background-color: #020617;
    color: #f1f5f9;
    border-color: #334155;
  }

  :global(.dark) pre[class*="language-"]:focus {
    border-color: #06b6d4;
    box-shadow: 0 0 0 2px rgba(6, 182, 212, 0.3);
  }

  pre[class*="language-"] :global(.token) {
    background: none !important;
  }

  /* Light mode token colors */
  pre[class*="language-"] :global(.token.comment),
  pre[class*="language-"] :global(.token.prolog) {
    color: #64748b;
  }
  pre[class*="language-"] :global(.token.keyword) {
    color: #7c3aed;
  }
  pre[class*="language-"] :global(.token.string) {
    color: #16a34a;
  }
  pre[class*="language-"] :global(.token.number) {
    color: #d97706;
  }
  pre[class*="language-"] :global(.token.regex),
  pre[class*="language-"] :global(.token.important) {
    color: #dc2626;
  }
  pre[class*="language-"] :global(.token.url) {
    color: #0284c7;
  }
  pre[class*="language-"] :global(.token.function) {
    color: #0284c7;
  }

  /* Dark mode token overrides */
  :global(.dark) pre[class*="language-"] :global(.token.comment),
  :global(.dark) pre[class*="language-"] :global(.token.prolog) {
    color: #94a3b8;
  }
  :global(.dark) pre[class*="language-"] :global(.token.keyword) {
    color: #c084fc;
  }
  :global(.dark) pre[class*="language-"] :global(.token.string) {
    color: #86efac;
  }
  :global(.dark) pre[class*="language-"] :global(.token.number) {
    color: #fbbf24;
  }
  :global(.dark) pre[class*="language-"] :global(.token.regex),
  :global(.dark) pre[class*="language-"] :global(.token.important) {
    color: #f87171;
  }
  :global(.dark) pre[class*="language-"] :global(.token.url) {
    color: #38bdf8;
  }
  :global(.dark) pre[class*="language-"] :global(.token.function) {
    color: #38bdf8;
  }
</style>
