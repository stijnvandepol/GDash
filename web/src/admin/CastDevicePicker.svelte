<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type CastDevice, type Display } from './api';
  import { deviceChoices } from './castDevices';

  let {
    selected,
    max,
    onchange,
  }: {
    /** The device IDs the widget follows; empty for any device. */
    selected: string[];
    max: number;
    onchange: (ids: string[]) => void;
  } = $props();

  let devices = $state<CastDevice[]>([]);
  let displays = $state<Display[]>([]);
  let searching = $state(true);
  let loadError = $state('');
  // "These devices" stays chosen while none is ticked yet.
  let pickDevices = $state(false);
  const onlyThese = $derived(pickDevices || selected.length > 0);

  const choices = $derived(deviceChoices(devices, displays, selected));

  async function search() {
    searching = true;
    loadError = '';
    try {
      [devices, displays] = await Promise.all([api.castDevices(), api.displays()]);
    } catch (err) {
      loadError = err instanceof Error ? err.message : String(err);
    } finally {
      searching = false;
    }
  }

  onMount(() => void search());

  function toggle(id: string, on: boolean) {
    onchange(on ? [...selected, id].slice(0, max) : selected.filter((s) => s !== id));
  }

  function chooseAny() {
    pickDevices = false;
    onchange([]);
  }
</script>

<fieldset class="picker">
  <legend>Cast devices</legend>
  <label class="option">
    <input type="radio" name="now-playing-devices" checked={!onlyThese} onchange={chooseAny} />
    Any device that is playing
  </label>
  <label class="option">
    <input type="radio" name="now-playing-devices" checked={onlyThese} onchange={() => (pickDevices = true)} />
    Only these devices
  </label>
  {#if onlyThese}
    {#if loadError}
      <p class="error">{loadError}</p>
    {:else if searching && choices.length === 0}
      <p class="muted small">Searching for Cast devices…</p>
    {:else}
      <ul>
        {#each choices as c (c.id)}
          {@const checked = selected.includes(c.id)}
          <li>
            <label class:blocked={c.blockedBy !== undefined}>
              <input
                type="checkbox"
                {checked}
                disabled={c.blockedBy !== undefined || (!checked && selected.length >= max)}
                onchange={(e) => toggle(c.id, (e.target as HTMLInputElement).checked)}
              />
              <span class="grow">{c.name} <span class="muted small">{c.detail}</span></span>
            </label>
            {#if c.blockedBy !== undefined}
              <span class="muted small">Shows the dashboard of {c.blockedBy}</span>
            {/if}
          </li>
        {:else}
          <li class="muted">No Cast devices found.</li>
        {/each}
      </ul>
      {#if selected.length === 0}
        <p class="field-error">Tick at least one device, or choose any device.</p>
      {/if}
    {/if}
    <p class="muted small">
      {#if !searching}<button type="button" class="link" onclick={search}>Search again</button>{/if}
      At most {max}. GDash only reads what plays; it never controls these devices.
    </p>
  {/if}
</fieldset>

<style>
  .picker {
    display: grid;
    gap: 6px;
    min-width: 0;
    margin: 0;
    padding: 0;
    border: 0;
  }
  legend {
    padding: 0;
    margin-bottom: 4px;
  }
  .option {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 400;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 4px;
    max-height: 14em;
    overflow-y: auto;
  }
  li {
    display: grid;
    gap: 2px;
    overflow-wrap: anywhere;
  }
  li label {
    display: flex;
    gap: 8px;
    align-items: baseline;
    font-weight: 400;
  }
  .blocked {
    opacity: 0.6;
  }
  .grow {
    flex: 1 1 auto;
    min-width: 0;
  }
</style>
