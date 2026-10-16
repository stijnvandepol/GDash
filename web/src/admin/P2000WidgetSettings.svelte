<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from './api';
  import { DEFAULT_DISCIPLINES, DISCIPLINES, REGION_NAMES, readPlaces, type Place } from '../widgets/p2000';

  // A region and places exclude each other (internal/display), so the P2000
  // widget has its own form: one switch between them, and a Test button
  // that finds each place's slug and region on alarmeringen.nl.
  const MAX_PLACES = 5;
  const MAX_HOURS = 48;
  const MAX_ITEMS = 20;
  const DISCIPLINE_LABELS: Record<string, string> = {
    brandweer: 'Fire brigade',
    politie: 'Police',
    ambulance: 'Ambulance (most alerts)',
    knrm: 'Lifeboat (KNRM)',
    trauma: 'Trauma helicopter',
  };

  let {
    settings,
    onchange,
  }: {
    settings: Record<string, unknown>;
    onchange: (settings: Record<string, unknown>) => void;
  } = $props();

  const places = $derived(readPlaces(settings.places));
  // What the admin chose here; until then, what is saved.
  let choice = $state<'region' | 'places' | undefined>(undefined);
  // The region before a switch to places, so switching back restores it.
  let lastRegion = $state<string | undefined>(undefined);
  const byPlaces = $derived((choice ?? (typeof settings.region === 'string' ? 'region' : 'places')) === 'places');
  const disciplines = $derived(
    Array.isArray(settings.disciplines) ? settings.disciplines.filter((d): d is string => typeof d === 'string') : [...DEFAULT_DISCIPLINES],
  );

  let name = $state('');
  let testing = $state(false);
  let problem = $state('');
  let found = $state('');

  // The weather location is the likeliest place; its name is offered, not added.
  onMount(async () => {
    try {
      const { settings: saved } = await api.settings();
      if (name === '' && saved.location) name = saved.location.name.split(',')[0]!.trim();
    } catch {
      // Without it the field starts empty.
    }
  });

  function choose(next: 'region' | 'places') {
    choice = next;
    const out = { ...settings };
    if (next === 'places') {
      if (typeof out.region === 'string') lastRegion = out.region;
      delete out.region;
    } else {
      delete out.places;
      if (typeof out.region !== 'string') out.region = lastRegion ?? places[0]?.region ?? 'utrecht';
    }
    onchange(out);
  }

  async function test() {
    problem = '';
    found = '';
    testing = true;
    try {
      const place = await api.testP2000Place(name);
      if (places.some((p) => p.slug === place.slug)) {
        problem = `${place.name} is already in the list.`;
      } else {
        const next: Place[] = [...places, { name: place.name, slug: place.slug, region: place.region }];
        const out: Record<string, unknown> = { ...settings, places: next };
        delete out.region;
        onchange(out);
        found = `${place.name} found, in ${place.regionName}.`;
        name = '';
      }
    } catch (err) {
      problem =
        err instanceof ApiError
          ? (err.fieldMessage('name') ?? err.message)
          : 'The place could not be checked. Try again in a moment.';
    } finally {
      testing = false;
    }
  }

  function remove(slug: string) {
    const next = places.filter((p) => p.slug !== slug);
    const out = { ...settings };
    if (next.length > 0) out.places = next;
    else delete out.places;
    onchange(out);
  }

  function toggle(discipline: string, on: boolean) {
    const chosen = DISCIPLINES.filter((d) => (d === discipline ? on : disciplines.includes(d)));
    // At least one: the last one cannot be switched off.
    if (chosen.length === 0) return;
    onchange({ ...settings, disciplines: chosen });
  }

  function setNumber(key: string, raw: string, max: number) {
    const n = Number(raw);
    const out = { ...settings };
    if (raw === '' || !Number.isInteger(n)) delete out[key];
    else out[key] = Math.min(Math.max(n, 1), max);
    onchange(out);
  }
</script>

<fieldset class="group">
  <legend>Area</legend>
  <label class="radio"><input type="radio" checked={!byPlaces} onchange={() => choose('region')} /> A safety region</label>
  <label class="radio"><input type="radio" checked={byPlaces} onchange={() => choose('places')} /> Places</label>
  {#if !byPlaces}
    <label class="inline-label">
      Safety region
      <select value={typeof settings.region === 'string' ? settings.region : ''} onchange={(e) => onchange({ ...settings, region: (e.target as HTMLSelectElement).value })}>
        {#each Object.entries(REGION_NAMES) as [slug, regionName] (slug)}
          <option value={slug}>{regionName}</option>
        {/each}
      </select>
    </label>
  {:else}
    {#if places.length > 0}
      <ul class="places">
        {#each places as place (place.slug)}
          <li>
            {place.name} <span class="muted small">{REGION_NAMES[place.region] ?? place.region}</span>
            <button type="button" class="link" onclick={() => remove(place.slug)} aria-label="Remove {place.name}">Remove</button>
          </li>
        {/each}
      </ul>
    {/if}
    {#if places.length < MAX_PLACES}
      <form
        class="inline-label"
        onsubmit={(e) => {
          e.preventDefault();
          void test();
        }}
      >
        <input aria-label="Place" placeholder="Nuenen" maxlength="100" bind:value={name} />
        <button type="submit" disabled={testing || name.trim().length < 2}>{testing ? 'Testing…' : 'Test and add'}</button>
      </form>
    {/if}
    {#if problem}
      <span class="field-error" role="alert">{problem}</span>
    {:else if found}
      <span class="small muted" role="status">{found}</span>
    {:else if places.length === 0}
      <span class="field-error">Add at least one place.</span>
    {/if}
  {/if}
</fieldset>

<fieldset class="group">
  <legend>Show</legend>
  {#each DISCIPLINES as discipline (discipline)}
    <label class="radio">
      <input type="checkbox" checked={disciplines.includes(discipline)} onchange={(e) => toggle(discipline, (e.target as HTMLInputElement).checked)} />
      {DISCIPLINE_LABELS[discipline]}
    </label>
  {/each}
</fieldset>

<label class="inline-label">
  Hours back
  <input
    type="number"
    min="1"
    max={MAX_HOURS}
    value={typeof settings.hours === 'number' ? settings.hours : ''}
    onchange={(e) => setNumber('hours', (e.target as HTMLInputElement).value, MAX_HOURS)}
  />
</label>
<label class="inline-label">
  Alerts at most
  <input
    type="number"
    min="1"
    max={MAX_ITEMS}
    value={typeof settings.maxItems === 'number' ? settings.maxItems : ''}
    onchange={(e) => setNumber('maxItems', (e.target as HTMLInputElement).value, MAX_ITEMS)}
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
  .places {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 4px;
  }
</style>
