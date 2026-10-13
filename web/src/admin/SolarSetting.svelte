<script lang="ts">
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { api, ApiError, type Location, type Solar, type SourceStatus } from './api';
  import { describeStatus } from './status';
  import { MAX_ARRAYS, SOLAR_SOURCES, copySolar, describeArray, newArray, newSolar } from './solar';
  import PlacePicker from './PlacePicker.svelte';

  let {
    panels,
    weatherLocation,
    status,
    onchanged,
    onerror,
  }: {
    panels: Solar | null;
    weatherLocation: Location | null;
    /** Present once a dashboard shows a solar forecast. */
    status: SourceStatus | undefined;
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  // Opening the card without panels goes straight to the form.
  // svelte-ignore state_referenced_locally
  let editing = $state(panels === null);
  let form = $state<Solar>(newSolar());
  let fieldErrors = $state<Record<string, string>>({});
  let error = $state('');
  let busy = $state(false);

  function edit() {
    form = panels ? copySolar(panels) : newSolar();
    fieldErrors = {};
    error = '';
    editing = true;
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = '';
    fieldErrors = {};
    try {
      await api.setSolar(form);
      toast.success('The solar panels are saved.');
      editing = false;
      await onchanged();
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = Object.fromEntries(err.fields.map((f) => [f.field, f.message]));
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  async function remove() {
    const message = 'Solar forecast widgets then say that no panels are set up.';
    if (!(await ask({ title: 'Remove the solar panels?', message, action: 'Remove' }))) return;
    error = '';
    try {
      await api.clearSolar();
      toast.success('The solar panels are removed.');
      await onchanged();
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card">
  <h2>Solar panels</h2>
  <p class="muted">
    For the Solar forecast widget: what your panels are expected to produce today and tomorrow. It is an estimate from
    the cloud forecast, not a measurement, so a changeable day can turn out quite different.
  </p>
  {#if panels && !editing}
    <ul class="list">
      <li>
        <div class="grow">
          {#each panels.arrays as a, i (i)}<strong class="array">{describeArray(a)}</strong>{/each}
          <span class="muted small">
            {panels.location ? `At ${panels.location.name}` : 'At the weather location'} · estimate from
            {panels.source === 'open-meteo' ? `Open-Meteo, performance ratio ${panels.performanceRatio}` : 'Forecast.Solar'}
          </span>
        </div>
        <button class="link" onclick={edit}>Change</button>
        <button class="link danger" onclick={remove}>Remove</button>
      </li>
    </ul>
    {#if !panels.location && !weatherLocation}
      <p class="small error">Set the weather location under Weather, or choose a place for the panels, before the forecast can be fetched.</p>
    {:else if status}
      <p class="small" class:error={status.failing}>{describeStatus(status)}</p>
    {:else}
      <p class="small muted">Add a Solar forecast widget to a dashboard to fetch the estimate.</p>
    {/if}
  {:else if !editing}
    <button onclick={edit}>Set up solar panels</button>
  {:else}
    <form onsubmit={save} class="solar-form">
      <PlacePicker
        name={form.location?.name ?? ''}
        onpick={(place) => (form.location = { name: place.name, latitude: place.latitude, longitude: place.longitude })}
        onclear={() => (form.location = null)}
      />
      {#if fieldErrors['location.latitude'] || fieldErrors['location.name']}
        <span class="field-error">{fieldErrors['location.latitude'] ?? fieldErrors['location.name']}</span>
      {/if}

      {#each form.arrays as a, i (i)}
        <fieldset>
          <legend>{form.arrays.length > 1 ? `Panels ${i + 1}` : 'Panels'}</legend>
          <div class="row">
            <label>
              Tilt (°)
              <input type="number" bind:value={a.tilt} min="0" max="90" step="1" required />
              <span class="small muted">0 is flat, 90 upright</span>
              {#if fieldErrors[`arrays[${i}].tilt`]}<span class="field-error">{fieldErrors[`arrays[${i}].tilt`]}</span>{/if}
            </label>
            <label>
              Direction (°)
              <input type="number" bind:value={a.azimuth} min="-180" max="180" step="1" required />
              <span class="small muted">0 south, -90 east, 90 west</span>
              {#if fieldErrors[`arrays[${i}].azimuth`]}<span class="field-error">{fieldErrors[`arrays[${i}].azimuth`]}</span>{/if}
            </label>
            <label>
              Peak power (kWp)
              <input type="number" bind:value={a.kwp} min="0.1" max="100" step="0.01" required />
              <span class="small muted">All panels in this group</span>
              {#if fieldErrors[`arrays[${i}].kwp`]}<span class="field-error">{fieldErrors[`arrays[${i}].kwp`]}</span>{/if}
            </label>
          </div>
          {#if form.arrays.length > 1}
            <button type="button" class="link danger" onclick={() => (form.arrays = form.arrays.filter((_, j) => j !== i))}>Remove these panels</button>
          {/if}
        </fieldset>
      {/each}
      {#if fieldErrors.arrays}<span class="field-error">{fieldErrors.arrays}</span>{/if}
      {#if form.arrays.length < MAX_ARRAYS}
        <button type="button" class="link" onclick={() => (form.arrays = [...form.arrays, newArray()])}>Add panels facing another way</button>
      {/if}

      <label>
        Estimate from
        <select bind:value={form.source}>
          {#each SOLAR_SOURCES as s (s.value)}<option value={s.value}>{s.label}</option>{/each}
        </select>
        {#if fieldErrors.source}<span class="field-error">{fieldErrors.source}</span>{/if}
      </label>
      {#if form.source === 'open-meteo'}
        <label>
          Performance ratio
          <input type="number" bind:value={form.performanceRatio} min="0.5" max="1" step="0.01" required />
          <span class="small muted">
            The share of the sunlight model that reaches your meter after inverter, heat and dirt losses. 0.85 suits most
            systems; compare a few sunny days with your inverter's numbers to fine-tune it.
          </span>
          {#if fieldErrors.performanceRatio}<span class="field-error">{fieldErrors.performanceRatio}</span>{/if}
        </label>
      {:else}
        <p class="small muted">
          Forecast.Solar models the panels itself and allows 12 requests an hour from your address; GDash asks every
          30 minutes, once per group of panels.
        </p>
      {/if}
      <div class="inline">
        <button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
        <button type="button" class="link" onclick={() => (editing = false)}>Cancel</button>
      </div>
    </form>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .array {
    display: block;
  }
  .solar-form {
    display: grid;
    gap: var(--s-3);
  }
  fieldset {
    display: grid;
    gap: var(--s-2);
    margin: 0;
    padding: 0;
    border: 0;
    max-width: 40rem;
  }
  legend {
    padding: 0;
    margin-bottom: var(--s-2);
    font-size: var(--fs-sm);
    font-weight: 550;
    color: var(--text-soft);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-4);
  }
  .row label {
    flex: 1 1 9rem;
  }
</style>
