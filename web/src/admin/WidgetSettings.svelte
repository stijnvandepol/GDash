<script lang="ts">
  import type { CustomSource, DepartureStop, Place, WidgetType } from './api';
  import { presetWidget } from './presetWidget';
  import PresetWidgetSettings from './PresetWidgetSettings.svelte';
  import { CHOICE_LABELS, CHOICE_UNSET, placeSetting, settingLabel } from './widgetSettings';
  import EntityPicker from './EntityPicker.svelte';
  import HomeyCapabilityPicker from './HomeyCapabilityPicker.svelte';
  import HomeyUserPicker from './HomeyUserPicker.svelte';
  import { presenceUsers } from './homey';
  import CalendarPicker from './CalendarPicker.svelte';
  import CameraPicker from './CameraPicker.svelte';
  import WebPagePicker from './WebPagePicker.svelte';
  import StopPicker from './StopPicker.svelte';
  import PlacePicker from './PlacePicker.svelte';
  import CastDevicePicker from './CastDevicePicker.svelte';
  import NewsFeedPicker from './NewsFeedPicker.svelte';
  import CustomWidgetSettings from './CustomWidgetSettings.svelte';
  import TrafficWidgetSettings from './TrafficWidgetSettings.svelte';

  // Texts up to this length get a single-line field, longer ones a text area.
  const SINGLE_LINE_MAX = 60;

  let {
    widgetType,
    type,
    settings,
    sources,
    onchange,
  }: {
    /** The widget's type id, such as news, for wording the labels. */
    widgetType: string;
    type: WidgetType;
    settings: Record<string, unknown>;
    /** The custom sources, null while they load. */
    sources: CustomSource[] | null;
    /** Called with the full new settings after every edit. */
    onchange: (settings: Record<string, unknown>) => void;
  } = $props();

  // The stop search stores the name beside the code, so it has no field.
  // Where comes first: the radius and the fuel are about the place, and
  // announcing is about the province.
  const isWhere = (key: string) => key === 'place' || key === 'region';
  const keys = $derived(
    Object.keys(type.settings)
      .filter((key) => !(key === 'stopName' && 'stop' in type.settings))
      .sort()
      .sort((a, b) => Number(isWhere(b)) - Number(isWhere(a))),
  );

  // The Custom widget's settings depend on each other, so it has its own
  // form; one that shows a device or service with a preset has the preset's.
  const isCustom = $derived(type.settings.source?.kind === 'customSource');
  const preset = $derived(isCustom && sources ? presetWidget(settings, sources) : undefined);

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

  function ids(key: string): number[] {
    const v = settings[key];
    return Array.isArray(v) ? v.filter((n): n is number => typeof n === 'number') : [];
  }

  function text(key: string): string {
    const v = settings[key];
    return typeof v === 'string' ? v : '';
  }
</script>

{#if isCustom && sources === null}
  <p class="small muted" role="status">Loading…</p>
{:else if preset}
  <PresetWidgetSettings preset={preset.preset} sources={preset.sources} {settings} {onchange} />
{:else if isCustom}
  <CustomWidgetSettings {settings} {onchange} />
{:else if type.settings.roads?.kind === 'roads'}
  <TrafficWidgetSettings {settings} {onchange} />
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
  {:else if rule.kind === 'camera'}
    <CameraPicker selected={typeof settings[key] === 'number' ? settings[key] : undefined} onchange={(id) => set(key, id)} />
  {:else if rule.kind === 'webPage'}
    <WebPagePicker selected={typeof settings[key] === 'number' ? settings[key] : undefined} onchange={(id) => set(key, id)} />
  {:else if rule.kind === 'stop'}
    <StopPicker code={text(key)} name={text('stopName')} onpick={pickStop} />
  {:else if rule.kind === 'place'}
    <PlacePicker name={placeName(key)} onpick={(place) => pickPlace(key, place)} onclear={() => set(key, undefined)} />
  {:else if rule.kind === 'boolean'}
    <label class="inline-label">
      <input type="checkbox" checked={settings[key] === true} onchange={(e) => set(key, (e.target as HTMLInputElement).checked)} />
      {settingLabel(widgetType, key)}
    </label>
  {:else if rule.kind === 'newsFeeds'}
    <!-- No feeds means every feed, which the server stores as no setting. -->
    <NewsFeedPicker selected={ids(key)} max={rule.max ?? 1} onchange={(chosen) => set(key, chosen.length > 0 ? chosen : undefined)} />
  {:else if rule.kind === 'castDevices'}
    <!-- No devices means any device, which the server stores as no setting. -->
    <CastDevicePicker selected={list(key)} max={rule.max ?? 1} onchange={(ids) => set(key, ids.length > 0 ? ids : undefined)} />
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
        {#if CHOICE_UNSET[key]}<option value="">{CHOICE_UNSET[key]}</option>{/if}
        {#each rule.options ?? [] as option (option)}
          <option value={option}>{CHOICE_LABELS[option] ?? option}</option>
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
