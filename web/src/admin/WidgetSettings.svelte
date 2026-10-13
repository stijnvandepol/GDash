<script lang="ts">
  import type { DepartureStop, Place, WidgetType } from './api';
  import { FUEL_LABELS, placeSetting, settingLabel } from './widgetSettings';
  import EntityPicker from './EntityPicker.svelte';
  import HomeyCapabilityPicker from './HomeyCapabilityPicker.svelte';
  import HomeyUserPicker from './HomeyUserPicker.svelte';
  import { presenceUsers } from './homey';
  import CalendarPicker from './CalendarPicker.svelte';
  import StopPicker from './StopPicker.svelte';
  import PlacePicker from './PlacePicker.svelte';
  import CustomWidgetSettings from './CustomWidgetSettings.svelte';

  // Texts up to this length get a single-line field, longer ones a text area.
  const SINGLE_LINE_MAX = 60;

  let {
    widgetType,
    type,
    settings,
    onchange,
  }: {
    /** The widget's type id, such as news, for wording the labels. */
    widgetType: string;
    type: WidgetType;
    settings: Record<string, unknown>;
    /** Called with the full new settings after every edit. */
    onchange: (settings: Record<string, unknown>) => void;
  } = $props();

  // The stop search stores the name beside the code, so it has no field.
  // A place comes first: the radius and the fuel are about it.
  const keys = $derived(
    Object.keys(type.settings)
      .filter((key) => !(key === 'stopName' && 'stop' in type.settings))
      .sort()
      .sort((a, b) => Number(b === 'place') - Number(a === 'place')),
  );

  // The Custom widget's settings depend on each other, so it has its own form.
  const isCustom = $derived(type.settings.source?.kind === 'customSource');

  function pickStop(stop: DepartureStop) {
    onchange({ ...settings, stop: stop.code, stopName: stop.name });
  }

  function pickPlace(key: string, place: Place) {
    onchange({ ...settings, [key]: placeSetting(place) });
  }

  function placeName(key: string): string {
    const v = settings[key];
    const name = typeof v === 'object' && v !== null ? (v as Record<string, unknown>).name : undefined;
    return typeof name === 'string' ? name : '';
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

{#if isCustom}
  <CustomWidgetSettings {settings} {onchange} />
{:else}
{#each keys as key (key)}
  {@const rule = type.settings[key]!}
  {#if rule.kind === 'entities'}
    <EntityPicker selected={list(key)} max={rule.max ?? 1} onchange={(ids) => set(key, ids)} />
  {:else if rule.kind === 'homeyCapabilities'}
    <HomeyCapabilityPicker selected={list(key)} max={rule.max ?? 1} onchange={(keys) => set(key, keys)} />
  {:else if rule.kind === 'homeyUsers'}
    <HomeyUserPicker selected={presenceUsers(settings[key])} max={rule.max ?? 1} onchange={(users) => set(key, users)} />
  {:else if rule.kind === 'calendar'}
    <CalendarPicker selected={typeof settings[key] === 'number' ? settings[key] : undefined} onchange={(id) => set(key, id)} />
  {:else if rule.kind === 'stop'}
    <StopPicker code={text(key)} name={text('stopName')} onpick={pickStop} />
  {:else if rule.kind === 'place'}
    <PlacePicker name={placeName(key)} onpick={(place) => pickPlace(key, place)} onclear={() => set(key, undefined)} />
  {:else}
  <label class="inline-label">
    {settingLabel(widgetType, key)}
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
    {:else if rule.kind === 'choice'}
      <select value={text(key)} onchange={(e) => set(key, (e.target as HTMLSelectElement).value)}>
        {#each rule.options ?? [] as option (option)}
          <option value={option}>{FUEL_LABELS[option] ?? option}</option>
        {/each}
      </select>
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
{/if}

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
