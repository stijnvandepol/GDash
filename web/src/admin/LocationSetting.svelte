<script lang="ts">
  import { api, ApiError, type Location, type Place, type SourceStatus } from './api';
  import { describeStatus } from './status';

  let {
    location,
    status,
    onchanged,
    onerror,
  }: {
    location: Location | null;
    status: SourceStatus | undefined;
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  // svelte-ignore state_referenced_locally
  let editing = $state(location === null);
  let manual = $state(false);
  let query = $state('');
  let places = $state<Place[] | null>(null);
  let searching = $state(false);
  let name = $state('');
  let latitude = $state('');
  let longitude = $state('');
  let error = $state('');

  async function search(event: SubmitEvent) {
    event.preventDefault();
    searching = true;
    error = '';
    try {
      places = await api.places(query.trim());
    } catch (err) {
      places = null;
      error = err instanceof ApiError ? (err.fieldMessage('name') ?? err.message) : onerror(err);
    } finally {
      searching = false;
    }
  }

  async function choose(loc: Location) {
    error = '';
    try {
      await api.setLocation(loc);
      editing = false;
      places = null;
      query = '';
      await onchanged();
    } catch (err) {
      error =
        err instanceof ApiError && err.fields.length > 0
          ? err.fields.map((f) => `${f.field}: ${f.message}`).join('; ')
          : onerror(err);
    }
  }

  /** A coordinate as typed, with a comma or a point as decimal separator. */
  function coordinate(text: string): number {
    return Number(text.trim().replace(',', '.'));
  }

  function chooseManual(event: SubmitEvent) {
    event.preventDefault();
    const lat = coordinate(latitude);
    const lon = coordinate(longitude);
    if (!Number.isFinite(lat) || !Number.isFinite(lon)) {
      error = 'Enter the latitude and longitude as numbers, for example 51.77 and 5.52.';
      return;
    }
    void choose({ name: name.trim(), latitude: lat, longitude: lon });
  }

  async function clear() {
    if (!confirm('Remove the location? Weather widgets then show that no location is set.')) return;
    try {
      await api.clearLocation();
      await onchanged();
      editing = true;
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card">
  <h2>Weather location</h2>
  {#if location && !editing}
    <p>
      <strong>{location.name}</strong>
      <span class="muted">({location.latitude.toFixed(2)}, {location.longitude.toFixed(2)})</span>
    </p>
    <p class="small" class:error={status?.failing}>{describeStatus(status)}</p>
    <div class="inline">
      <button onclick={() => (editing = true)}>Change</button>
      <button class="link danger" onclick={clear}>Remove</button>
    </div>
  {:else}
    <p class="muted">
      The weather is fetched for this place, rounded to about 1 km before it leaves your server.
    </p>
    {#if manual}
      <form onsubmit={chooseManual}>
        <label>Name <input bind:value={name} placeholder="Home" required maxlength="60" /></label>
        <label>Latitude <input bind:value={latitude} inputmode="decimal" placeholder="51.77" required /></label>
        <label>Longitude <input bind:value={longitude} inputmode="decimal" placeholder="5.52" required /></label>
        <div class="inline">
          <button type="submit">Save</button>
          <button type="button" class="link" onclick={() => (manual = false)}>Search by name instead</button>
        </div>
      </form>
    {:else}
      <form class="inline" onsubmit={search}>
        <input bind:value={query} aria-label="Place name" placeholder="Town or postcode" required minlength="2" />
        <button type="submit" disabled={searching}>Search</button>
        <button type="button" class="link" onclick={() => (manual = true)}>Enter coordinates</button>
      </form>
      {#if places && places.length === 0}
        <p class="muted">No places found. Try another spelling, or enter the coordinates.</p>
      {:else if places}
        <ul class="list">
          {#each places as place, i (i)}
            <li>
              <div class="grow">
                <strong>{place.name}</strong>
                <span class="muted">{place.region}</span>
              </div>
              <button onclick={() => choose({ name: place.name, latitude: place.latitude, longitude: place.longitude })}>
                Use this
              </button>
            </li>
          {/each}
        </ul>
      {/if}
      <p class="small muted">Place search by Open-Meteo, with data from GeoNames.</p>
    {/if}
    {#if location}
      <button class="link" onclick={() => (editing = false)}>Cancel</button>
    {/if}
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>
