<script lang="ts">
  import 'gridstack/dist/gridstack.min.css';
  import { GridStack, type GridStackNode, type GridStackWidget } from 'gridstack';
  import { onDestroy, onMount } from 'svelte';
  import { api, ApiError, type Dashboard, type Widget, type WidgetCatalogue } from './api';

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

  const LABELS: Record<string, string> = { clock: 'Clock', weather: 'Weather', news: 'News', calendar: 'Calendar' };

  // Widgets by id; positions live in the grid until saving.
  let widgets = $state<Record<string, Widget>>({});
  let name = $state('');
  let selected = $state<string | null>(null);
  let message = $state('');
  let errors = $state<string[]>([]);
  let saving = $state(false);
  let dirty = $state(false);

  let container: HTMLDivElement;
  let grid: GridStack | null = null;
  let resizeObserver: ResizeObserver | null = null;

  const selectedWidget = $derived(selected ? widgets[selected] : undefined);
  const selectedType = $derived(selectedWidget ? catalogue.types[selectedWidget.type] : undefined);

  onMount(() => {
    name = dashboard.name;
    for (const w of dashboard.widgets) widgets[w.id] = { ...w, settings: { ...(w.settings ?? {}) } };

    // Plain text only: widget labels never become HTML.
    GridStack.renderCB = (el, w) => {
      el.textContent = w.content ?? '';
    };
    const created = GridStack.init(
      {
        column: catalogue.columns,
        row: catalogue.rows,
        float: true,
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
    widgets[id] = { id, type, x: 0, y: 0, w: t.minW, h: t.minH, settings: t.hasMaxItems ? { maxItems: 5 } : {} };
    grid.addWidget(node);
    selected = id;
    message = '';
    dirty = true;
  }

  function removeSelected() {
    if (!grid || !selected) return;
    const el = container.querySelector(`.grid-stack-item[gs-id="${CSS.escape(selected)}"]`);
    if (el) grid.removeWidget(el as HTMLElement);
    delete widgets[selected];
    selected = null;
    dirty = true;
  }

  function setMaxItems(value: number) {
    const w = selected ? widgets[selected] : undefined;
    if (!w) return;
    w.settings = { maxItems: value };
    dirty = true;
  }

  async function save() {
    if (!grid) return;
    saving = true;
    message = '';
    errors = [];
    const nodes = grid.save(false) as GridStackNode[];
    const layout: Widget[] = nodes.map((n) => {
      const w = widgets[String(n.id)]!;
      const settings = w.settings && Object.keys(w.settings).length > 0 ? w.settings : undefined;
      return { id: w.id, type: w.type, x: n.x ?? 0, y: n.y ?? 0, w: n.w ?? 1, h: n.h ?? 1, settings };
    });
    try {
      await api.saveDashboard({ id: dashboard.id, name: name.trim(), widgets: layout });
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

  function close() {
    if (dirty && !confirm('Leave without saving your changes?')) return;
    onClose();
  }
</script>

<section class="card">
  <div class="editor-head">
    <button class="link" onclick={close}>← Dashboards</button>
    <input class="name" bind:value={name} oninput={() => (dirty = true)} aria-label="Dashboard name" />
    <button onclick={save} disabled={saving}>{saving ? 'Saving…' : 'Save'}</button>
  </div>

  <div class="palette">
    <span class="muted small">Add:</span>
    {#each Object.keys(catalogue.types) as type (type)}
      <button class="chip" onclick={() => addWidget(type)}>{LABELS[type] ?? type}</button>
    {/each}
  </div>

  <div class="canvas">
    <div class="grid-stack" bind:this={container}></div>
  </div>
  <p class="muted small">Drag widgets to move them, drag a corner to resize. The canvas is the TV screen.</p>

  {#if selectedWidget}
    <div class="inspector">
      <strong>{LABELS[selectedWidget.type] ?? selectedWidget.type}</strong>
      {#if selectedType?.hasMaxItems}
        <label class="inline-label">
          Items
          <input
            type="number"
            min="1"
            max="20"
            value={Number(selectedWidget.settings?.maxItems ?? 5)}
            onchange={(e) => setMaxItems(Number((e.target as HTMLInputElement).value))}
          />
        </label>
      {/if}
      <button class="link danger" onclick={removeSelected}>Remove widget</button>
    </div>
  {/if}

  {#if message}<p class="muted">{message}</p>{/if}
  {#each errors as e (e)}<p class="error">{e}</p>{/each}
</section>

<style>
  .editor-head {
    display: flex;
    gap: 10px;
    align-items: center;
    flex-wrap: wrap;
  }
  .name {
    flex: 1;
    min-width: 160px;
    font-weight: 600;
  }
  .palette {
    display: flex;
    gap: 8px;
    align-items: center;
    flex-wrap: wrap;
  }
  .chip {
    padding: 6px 12px;
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--border);
  }
  .canvas {
    background: #0b0c0e;
    border-radius: 8px;
    aspect-ratio: 16 / 9;
    overflow: hidden;
  }
  .canvas :global(.grid-stack-item-content) {
    background: #1f232a;
    color: #e8eaee;
    border-radius: 6px;
    display: grid;
    place-items: center;
    font-size: 0.95rem;
    cursor: move;
  }
  .canvas :global(.grid-stack-item.ui-draggable-dragging .grid-stack-item-content),
  .canvas :global(.grid-stack-item.ui-resizable-resizing .grid-stack-item-content) {
    outline: 2px solid var(--accent);
  }
  .inspector {
    display: flex;
    gap: 14px;
    align-items: center;
    flex-wrap: wrap;
    padding: 10px 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
  }
  .inline-label {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 400;
  }
  .inline-label input {
    width: 4.5em;
  }
</style>
