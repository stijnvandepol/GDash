<script lang="ts">
  import EditorToolbar from './EditorToolbar.svelte';
  import { ask } from './confirm.svelte';
  import 'gridstack/dist/gridstack.min.css';
  import { GridStack, type GridStackNode, type GridStackWidget } from 'gridstack';
  import { onDestroy, onMount, tick } from 'svelte';
  import { api, ApiError, type Dashboard, type Widget, type WidgetCatalogue } from './api';
  import type { PreviewDraft } from '../previewApi';
  import DashboardPreview from './DashboardPreview.svelte';
  import WidgetInspector from './WidgetInspector.svelte';
  import WidgetPalette from './WidgetPalette.svelte';
  import { WIDGET_LABELS as LABELS, defaultSettings } from './widgetSettings';

  let {
    dashboard,
    catalogue,
    onClose,
    onSessionEnded,
  }: {
    dashboard: Dashboard;
    catalogue: WidgetCatalogue;
    onClose: () => void;
    onSessionEnded: () => void;
  } = $props();

  // Widgets by id; positions live in the grid until saving.
  let widgets = $state<Record<string, Widget>>({});
  let name = $state('');
  let theme = $state('minimal-dark');
  let font = $state('');
  let nightTheme = $state('');

  let selected = $state<string | null>(null);
  let message = $state('');
  let errors = $state<string[]>([]);
  let saving = $state(false);
  let dirty = $state(false);

  let container: HTMLDivElement;
  let side: HTMLElement;
  let grid: GridStack | null = null;
  let resizeObserver: ResizeObserver | null = null;

  const selectedWidget = $derived(selected ? widgets[selected] : undefined);
  const selectedType = $derived(selectedWidget ? catalogue.types[selectedWidget.type] : undefined);

  onMount(() => {
    name = dashboard.name;
    theme = dashboard.theme ?? 'minimal-dark';
    font = dashboard.font ?? '';
    nightTheme = dashboard.nightTheme ?? '';
    for (const w of dashboard.widgets) widgets[w.id] = { ...w, settings: { ...(w.settings ?? {}) } };

    // Plain text only: widget labels never become HTML.
    GridStack.renderCB = (el, w) => {
      el.textContent = w.content ?? '';
    };
    const created = GridStack.init(
      {
        column: catalogue.columns,
        row: catalogue.rows,
        // Widgets stay exactly where they are placed, as on the TV.
        mode: 'float',
        margin: 3,
        cellHeight: cellHeight(),
        animate: false,
      },
      container,
    );
    if (!created) {
      message = 'The editor could not start in this browser.';
      return;
    }
    grid = created;
    created.load(dashboard.widgets.map(toNode));
    created.on('change', () => (dirty = true));
    container.addEventListener('pointerdown', selectFromEvent);

    resizeObserver = new ResizeObserver(() => grid?.cellHeight(cellHeight()));
    resizeObserver.observe(container);
  });

  onDestroy(() => {
    resizeObserver?.disconnect();
    grid?.destroy(false);
  });

  /** Rows fill a 16:9 canvas as wide as the editor. */
  function cellHeight(): number {
    return Math.max(8, Math.floor((container.clientWidth * 9) / 16 / catalogue.rows));
  }

  function toNode(w: Widget): GridStackWidget {
    const type = catalogue.types[w.type];
    return { id: w.id, x: w.x, y: w.y, w: w.w, h: w.h, minW: type?.minW, minH: type?.minH, content: LABELS[w.type] ?? w.type };
  }

  // gridstack owns the widget elements, so the selection is marked on them
  // directly: the widget being edited must stand out on the canvas.
  $effect(() => {
    const id = selected;
    for (const el of container?.querySelectorAll<HTMLElement>('.grid-stack-item') ?? []) {
      el.classList.toggle('is-selected', el.getAttribute('gs-id') === id);
    }
  });

  function selectFromEvent(event: PointerEvent) {
    const item = (event.target as HTMLElement).closest<HTMLElement>('.grid-stack-item');
    selected = item?.getAttribute('gs-id') ?? null;
  }

  function addWidget(type: string) {
    if (!grid) return;
    const t = catalogue.types[type]!;
    let n = 1;
    while (widgets[`${type}-${n}`] || (n === 1 && widgets[type])) n++;
    const id = n === 1 ? type : `${type}-${n}`;
    const node: GridStackWidget = { ...toNode({ id, type, x: 0, y: 0, w: t.minW, h: t.minH }), autoPosition: true };
    if (!grid.willItFit(node)) {
      message = `There is no free space for a ${LABELS[type]?.toLowerCase() ?? type} widget. Make room first.`;
      return;
    }
    widgets[id] = { id, type, x: 0, y: 0, w: t.minW, h: t.minH, settings: defaultSettings(t) };
    grid.addWidget(node);
    selected = id;
    message = '';
    dirty = true;
    void focusSide('h2');
  }

  /** Back from a widget's settings to the widgets to add. */
  function deselect() {
    selected = null;
    void focusSide('input[type="search"]');
  }

  // The side panel changes what it shows; the keyboard follows it there
  // instead of falling back to the top of the page.
  async function focusSide(selector: string) {
    await tick();
    side.querySelector<HTMLElement>(selector)?.focus();
  }

  function removeSelected() {
    if (!grid || !selected) return;
    const el = container.querySelector(`.grid-stack-item[gs-id="${CSS.escape(selected)}"]`);
    if (el) grid.removeWidget(el as HTMLElement);
    delete widgets[selected];
    selected = null;
    dirty = true;
  }

  function setSettings(settings: Record<string, unknown>) {
    const w = selected ? widgets[selected] : undefined;
    if (!w) return;
    w.settings = settings;
    dirty = true;
  }

  /** The widgets as they are on the canvas now. */
  function currentLayout(): Widget[] {
    // The engine's nodes, not grid.save(): save() leaves out a width or
    // height that equals the widget's minimum, as a default to omit.
    const nodes: GridStackNode[] = grid?.engine.nodes ?? [];
    return nodes.map((n) => {
      const w = widgets[String(n.id)]!;
      const settings = w.settings && Object.keys(w.settings).length > 0 ? w.settings : undefined;
      return { id: w.id, type: w.type, x: n.x ?? 0, y: n.y ?? 0, w: n.w ?? 1, h: n.h ?? 1, settings };
    });
  }

  let previewing = $state(false);
  // The unsaved layout to preview; null shows the saved dashboard.
  let previewDraft = $state<PreviewDraft | null>(null);

  function openPreview() {
    previewDraft = dirty ? { name: name.trim(), theme, font, nightTheme, widgets: currentLayout() } : null;
    previewing = true;
  }

  async function save() {
    if (!grid) return;
    saving = true;
    message = '';
    errors = [];
    try {
      await api.saveDashboard({ id: dashboard.id, name: name.trim(), theme, font, nightTheme, widgets: currentLayout() });
      dirty = false;
      message = 'Saved. Screens showing this dashboard update now.';
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onSessionEnded();
        return;
      }
      errors = err instanceof ApiError && err.fields.length > 0 ? err.fields.map(describeField) : [err instanceof Error ? err.message : 'Saving failed.'];
    } finally {
      saving = false;
    }
  }

  /** Turns "widgets[2]: overlaps another widget" into "News: overlaps another widget". */
  function describeField(f: { field: string; message: string }): string {
    const match = /^widgets\[(\d+)\]$/.exec(f.field);
    if (!match || !grid) return `${f.field}: ${f.message}`;
    const node = (grid.save(false) as GridStackNode[])[Number(match[1])];
    const w = node ? widgets[String(node.id)] : undefined;
    return `${w ? (LABELS[w.type] ?? w.type) : f.field}: ${f.message}`;
  }

  async function close() {
    const message = 'Your changes to this dashboard are not saved yet.';
    if (dirty && !(await ask({ title: 'Leave without saving?', message, action: 'Leave' }))) return;
    onClose();
  }
</script>

<section class="card wide editor">
  <EditorToolbar
    bind:name
    bind:theme
    bind:font
    bind:nightTheme
    {catalogue}
    {saving}
    onchange={() => (dirty = true)}
    onclose={close}
    onpreview={openPreview}
    onsave={save}
  />

  <!-- The canvas, and beside it the widgets to add or the selected
       widget's settings; on a narrow screen the panel goes below. -->
  <div class="workspace">
    <div class="stage">
      <div class="canvas">
        <div class="grid-stack" bind:this={container}></div>
      </div>
      <p class="muted small">Drag widgets to move them, drag a corner to resize. The canvas is the TV screen.</p>
      {#if message}<p class="muted">{message}</p>{/if}
      {#each errors as e, i (i)}<p class="error">{e}</p>{/each}
    </div>

    <aside class="side" bind:this={side} aria-label={selectedWidget ? 'Widget settings' : 'Add widgets'}>
      {#if selectedWidget}
        {#key selected}
          <WidgetInspector
            label={LABELS[selectedWidget.type] ?? selectedWidget.type}
            type={selectedType}
            settings={selectedWidget.settings ?? {}}
            onchange={setSettings}
            onremove={removeSelected}
            onback={deselect}
          />
        {/key}
      {:else}
        <WidgetPalette types={Object.keys(catalogue.types)} onadd={addWidget} />
      {/if}
    </aside>
  </div>
</section>

{#if previewing}
  <DashboardPreview dashboardId={dashboard.id} draft={previewDraft} onClose={() => (previewing = false)} />
{/if}

<style>
  .editor {
    --editor-chrome: 300px;
    --side-w: 340px;
  }
  .workspace {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--s-5);
    align-items: start;
  }
  .stage {
    display: grid;
    gap: var(--s-3);
    min-width: 0;
  }
  .side {
    min-width: 0;
    padding-top: var(--s-4);
    border-top: 1px solid var(--rule);
  }
  /* Wide enough for both: the panel stands beside the TV and scrolls on its
     own, so the canvas stays in view while choosing. */
  @container page (min-width: 960px) {
    .workspace {
      grid-template-columns: minmax(0, 1fr) var(--side-w);
    }
    .side {
      position: sticky;
      top: calc(var(--topbar-h) + var(--s-4));
      /* As tall as the canvas when the window's height limits it, so both
         end on one line. */
      max-height: max(16rem, calc(100dvh - var(--editor-chrome)));
      overflow-y: auto;
      overscroll-behavior: contain;
      padding: 0 var(--s-2) var(--s-4) var(--s-5);
      border-top: 0;
      border-left: 1px solid var(--border);
    }
  }
  /* The canvas is the TV: dark, with the 24x14 grid drawn faintly so a
     widget's place and size can be judged while dragging. */
  .canvas {
    --cell-line: var(--device-line);
    background-color: var(--device-screen);
    background-image:
      linear-gradient(to right, var(--cell-line) 1px, transparent 1px),
      linear-gradient(to bottom, var(--cell-line) 1px, transparent 1px);
    background-size: calc(100% / 24) calc(100% / 14);
    border-radius: var(--r-md);
    aspect-ratio: 16 / 9;
    overflow: hidden;
    /* As wide as the card, but never taller than the window leaves room
       for: the toolbar above and the selected widget's settings below
       stay in view. */
    width: min(100%, calc((100dvh - var(--editor-chrome)) * 16 / 9));
    margin-inline: auto;
  }
  .canvas :global(.grid-stack-item-content) {
    background: var(--device-tile);
    color: var(--device-text);
    border: 1px solid var(--device-line);
    border-radius: var(--r-md);
    font-weight: 550;
    display: grid;
    place-items: center;
    font-size: 0.95rem;
    cursor: move;
  }
  .canvas :global(.grid-stack-item.is-selected .grid-stack-item-content) {
    background: var(--device-tile-selected);
    border-color: var(--spark);
    box-shadow: inset 0 0 0 1px var(--spark);
  }
  .canvas :global(.grid-stack-item.ui-draggable-dragging .grid-stack-item-content),
  .canvas :global(.grid-stack-item.ui-resizable-resizing .grid-stack-item-content) {
    outline: 2px solid var(--spark);
  }
</style>
