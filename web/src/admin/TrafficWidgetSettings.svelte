<script lang="ts">
  import { MAX_ROADS, TRAFFIC_KINDS, parseRoads } from './traffic';

  // The roads and the radius exclude each other (internal/display), so the
  // Traffic widget has its own form: one switch between them.
  const DEFAULT_RADIUS_KM = 25;
  const MAX_RADIUS_KM = 100;
  const MAX_LINES = 10;
  const DEFAULT_SHOW = ['jams', 'roadworks', 'speedchecks'];

  let {
    settings,
    onchange,
  }: {
    settings: Record<string, unknown>;
    onchange: (settings: Record<string, unknown>) => void;
  } = $props();

  const savedRoads = $derived(Array.isArray(settings.roads) ? settings.roads.filter((r): r is string => typeof r === 'string') : []);
  // What the admin chose and typed here; until then, what is saved. The
  // editor makes a new form for every widget it selects.
  let choice = $state<'roads' | 'near' | undefined>(undefined);
  let typed = $state<string | undefined>(undefined);
  let invalid = $state<string[]>([]);
  const watchRoads = $derived((choice ?? (savedRoads.length > 0 ? 'roads' : 'near')) === 'roads');
  const roadsText = $derived(typed ?? savedRoads.join(', '));

  const show = $derived(Array.isArray(settings.show) ? settings.show.filter((s): s is string => typeof s === 'string') : DEFAULT_SHOW);

  function chooseRoads(on: boolean) {
    choice = on ? 'roads' : 'near';
    const next = { ...settings };
    if (on) {
      delete next.radius;
      const { roads } = parseRoads(roadsText);
      if (roads.length > 0) next.roads = roads.slice(0, MAX_ROADS);
    } else {
      delete next.roads;
      next.radius = DEFAULT_RADIUS_KM;
    }
    onchange(next);
  }

  function typeRoads(text: string) {
    typed = text;
    const parsed = parseRoads(text);
    invalid = parsed.invalid;
    const next = { ...settings };
    delete next.radius;
    if (parsed.roads.length > 0) next.roads = parsed.roads.slice(0, MAX_ROADS);
    else delete next.roads;
    onchange(next);
  }

  function toggle(kind: string, on: boolean) {
    const chosen = TRAFFIC_KINDS.map(([k]) => k).filter((k) => (k === kind ? on : show.includes(k)));
    // At least one kind: the last one cannot be switched off.
    if (chosen.length === 0) return;
    onchange({ ...settings, show: chosen });
  }

  function setNumber(key: string, raw: string, max: number) {
    const n = Number(raw);
    const next = { ...settings };
    if (raw === '' || !Number.isInteger(n)) delete next[key];
    else next[key] = Math.min(Math.max(n, 1), max);
    onchange(next);
  }
</script>

<fieldset class="group">
  <legend>Watch</legend>
  <label class="radio"><input type="radio" checked={!watchRoads} onchange={() => chooseRoads(false)} /> Around the weather location</label>
  <label class="radio"><input type="radio" checked={watchRoads} onchange={() => chooseRoads(true)} /> My roads</label>
  {#if watchRoads}
    <label class="inline-label">
      Roads
      <input
        aria-label="Roads"
        placeholder="A2, A50, N279"
        value={roadsText}
        oninput={(e) => typeRoads((e.target as HTMLInputElement).value)}
      />
    </label>
    {#if invalid.length > 0}
      <span class="field-error">Not an A or N road: {invalid.join(', ')}</span>
    {:else if savedRoads.length === 0}
      <span class="field-error">Type at least one road, such as A2.</span>
    {/if}
  {:else}
    <label class="inline-label">
      Radius (km)
      <input
        type="number"
        min="1"
        max={MAX_RADIUS_KM}
        value={typeof settings.radius === 'number' ? settings.radius : DEFAULT_RADIUS_KM}
        onchange={(e) => setNumber('radius', (e.target as HTMLInputElement).value, MAX_RADIUS_KM)}
      />
    </label>
  {/if}
</fieldset>

<fieldset class="group">
  <legend>Show</legend>
  {#each TRAFFIC_KINDS as [kind, label] (kind)}
    <label class="radio">
      <input type="checkbox" checked={show.includes(kind)} onchange={(e) => toggle(kind, (e.target as HTMLInputElement).checked)} />
      {label}
    </label>
  {/each}
</fieldset>

<label class="inline-label">
  Lines at most
  <input
    type="number"
    min="1"
    max={MAX_LINES}
    value={typeof settings.maxItems === 'number' ? settings.maxItems : ''}
    onchange={(e) => setNumber('maxItems', (e.target as HTMLInputElement).value, MAX_LINES)}
  />
</label>

<style>
  .group {
    display: grid;
    gap: 6px;
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  legend {
    padding: 0;
    margin-bottom: 4px;
    font-weight: 550;
  }
  .radio,
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
  .inline-label input:not([type]) {
    flex: 1 1 10em;
    min-width: 0;
  }
</style>
