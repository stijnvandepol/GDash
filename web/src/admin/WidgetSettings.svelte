<script lang="ts">
  import type { DepartureStop, WidgetType } from './api';
  import { SETTING_LABELS } from './widgetSettings';
  import EntityPicker from './EntityPicker.svelte';
  import CalendarPicker from './CalendarPicker.svelte';
  import StopPicker from './StopPicker.svelte';

  // Texts up to this length get a single-line field, longer ones a text area.
  const SINGLE_LINE_MAX = 60;

  let {
    type,
    settings,
    onchange,
  }: {
    type: WidgetType;
    settings: Record<string, unknown>;
    /** Called with the full new settings after every edit. */
    onchange: (settings: Record<string, unknown>) => void;
  } = $props();

  // The stop search stores the name beside the code, so it has no field.
  const keys = $derived(
    Object.keys(type.settings)
      .filter((key) => !(key === 'stopName' && 'stop' in type.settings))
      .sort(),
  );

  function pickStop(stop: DepartureStop) {
    onchange({ ...settings, stop: stop.code, stopName: stop.name });
  }

  function set(key: string, value: unknown) {
    const next = { ...settings };
    if (value === '' || value === undefined) {
      delete next[key];
    } else {
      next[key] = value;
    }
    onchange(next);
  }

  function list(key: string): string[] {
    const v = settings[key];
    return Array.isArray(v) ? v.filter((s): s is string => typeof s === 'string') : [];
  }

  function text(key: string): string {
    const v = settings[key];
    return typeof v === 'string' ? v : '';
  }
</script>

{#each keys as key (key)}
  {@const rule = type.settings[key]!}
  {#if rule.kind === 'entities'}
    <EntityPicker selected={list(key)} max={rule.max ?? 1} onchange={(ids) => set(key, ids)} />
  {:else if rule.kind === 'calendar'}
    <CalendarPicker selected={typeof settings[key] === 'number' ? settings[key] : undefined} onchange={(id) => set(key, id)} />
  {:else if rule.kind === 'stop'}
    <StopPicker code={text(key)} name={text('stopName')} onpick={pickStop} />
  {:else}
  <label class="inline-label">
    {SETTING_LABELS[key] ?? key}
    {#if rule.kind === 'integer'}
      <input
        type="number"
        min={rule.min}
        max={rule.max}
        value={typeof settings[key] === 'number' ? settings[key] : ''}
        onchange={(e) => {
          const raw = (e.target as HTMLInputElement).value;
          set(key, raw === '' ? '' : Number(raw));
        }}
      />
    {:else if rule.kind === 'date'}
      <input type="date" value={text(key)} onchange={(e) => set(key, (e.target as HTMLInputElement).value)} />
    {:else if (rule.max ?? 0) > SINGLE_LINE_MAX}
      <textarea
        rows="3"
        maxlength={rule.max}
        value={text(key)}
        oninput={(e) => set(key, (e.target as HTMLTextAreaElement).value)}
      ></textarea>
    {:else}
      <input
        maxlength={rule.max}
        value={text(key)}
        oninput={(e) => set(key, (e.target as HTMLInputElement).value)}
      />
    {/if}
  </label>
  {/if}
{/each}

<style>
  .inline-label {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    font-weight: 400;
    min-width: 0;
  }
  input[type='number'] {
    width: 4.5em;
  }
  textarea {
    flex: 1 1 16em;
    min-width: 0;
    font: inherit;
  }
</style>
