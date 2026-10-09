<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError, type CastDevice, type Dashboard, type Display } from './api';
  import { errorMessage } from './errors';
  import { newestOnly } from './newest';
  import { describeCast } from './status';
  import ScheduleEditor from './ScheduleEditor.svelte';
  import AddDisplay from './AddDisplay.svelte';

  let { onSessionEnded }: { onSessionEnded: () => void } = $props();

  const STATUS_REFRESH_MS = 15_000;

  let displays = $state<Display[]>([]);
  let dashboards = $state<Dashboard[]>([]);
  let devices = $state<CastDevice[]>([]);
  let searching = $state(false);
  let searchMessage = $state('');
  let loadError = $state('');


  let editingId = $state<string | null>(null);
  let editName = $state('');
  let pairingId = $state<string | null>(null);
  let pairCode = $state('');
  let scheduleId = $state<string | null>(null);
  let rowError = $state('');

  const deviceNames = $derived(new Map(devices.map((d) => [d.id, d.name])));

  onMount(() => {
    void load();
    void search();
    // Keeps "On screen since" and similar current while the page is open.
    const timer = setInterval(() => void load(), STATUS_REFRESH_MS);
    return () => clearInterval(timer);
  });

  /** Sends the user back to the login screen when the session has ended. */
  const handle = (err: unknown) => errorMessage(err, onSessionEnded);

  const fetchLists = newestOnly(() => Promise.all([api.displays(), api.dashboards()]));

  async function load() {
    try {
      const newest = await fetchLists();
      if (!newest) return;
      [displays, dashboards] = newest.value;
      loadError = '';
    } catch (err) {
      loadError = handle(err);
    }
  }

  async function search() {
    searching = true;
    searchMessage = '';
    try {
      devices = await api.castDevices();
      if (devices.length === 0) searchMessage = 'No Cast devices found. Is the TV switched on and on the same network?';
    } catch (err) {
      searchMessage = handle(err);
    } finally {
      searching = false;
    }
  }

  function startRename(d: Display) {
    editingId = d.id;
    editName = d.name;
    rowError = '';
  }

  async function saveRename(event: SubmitEvent, id: string) {
    event.preventDefault();
    try {
      await api.updateDisplay(id, { name: editName.trim() });
      editingId = null;
      await load();
    } catch (err) {
      rowError = err instanceof ApiError ? (err.fieldMessage('name') ?? err.message) : handle(err);
    }
  }

  async function remove(d: Display) {
    if (!confirm(`Remove display "${d.name}"?`)) return;
    try {
      await api.deleteDisplay(d.id);
      await load();
    } catch (err) {
      rowError = handle(err);
    }
  }

  // Text sizes the server offers (internal/displays.TextScales).
  const TEXT_SIZES: [number, string][] = [
    [100, 'Text 100 % (TV)'],
    [125, 'Text 125 %'],
    [150, 'Text 150 % (small screen)'],
    [200, 'Text 200 %'],
  ];

  async function chooseTextScale(d: Display, textScale: number) {
    try {
      await api.updateDisplay(d.id, { textScale });
      await load();
    } catch (err) {
      rowError = err instanceof ApiError ? (err.fieldMessage('textScale') ?? err.message) : handle(err);
    }
  }

  async function chooseDashboard(d: Display, dashboardId: string) {
    try {
      await api.updateDisplay(d.id, { dashboardId: dashboardId === 'default' ? '' : dashboardId });
      await load();
    } catch (err) {
      rowError = err instanceof ApiError ? (err.fieldMessage('dashboardId') ?? err.message) : handle(err);
    }
  }

  function describeHours(d: Display): string {
    const hours = d.activeFrom ? `on ${d.activeFrom}–${d.activeUntil}` : 'always on';
    const n = d.schedule.length;
    return n === 0 ? hours : `${hours} · ${n === 1 ? '1 time slot' : `${n} time slots`}`;
  }

  async function closeSchedule(saved: boolean) {
    scheduleId = null;
    if (saved) await load();
  }

  function startPairing(d: Display) {
    pairingId = d.id;
    pairCode = '';
    rowError = '';
  }

  async function pair(event: SubmitEvent, id: string) {
    event.preventDefault();
    try {
      await api.pairScreen(id, pairCode.trim());
      pairingId = null;
      await load();
    } catch (err) {
      rowError = handle(err);
    }
  }

  async function unpair(d: Display) {
    if (!confirm(`Unpair every screen showing "${d.name}"? They will ask for a new code.`)) return;
    try {
      await api.unpairScreens(d.id);
      await load();
    } catch (err) {
      rowError = handle(err);
    }
  }

  function describeScreens(d: Display): string {
    const n = d.pairedScreens ?? 0;
    return n === 1 ? '1 paired screen' : `${n} paired screens`;
  }

  function describeDevice(id: string): string {
    if (!id) return 'Browser or kiosk';
    return deviceNames.get(id) ?? 'Cast device (not found right now)';
  }
</script>

<section class="card">
  <h1>Displays</h1>
  {#if loadError}<p class="error">{loadError}</p>{/if}
  {#if displays.length === 0 && !loadError}
    <p class="muted">No displays yet. Add your first TV below.</p>
  {:else}
    <ul class="list">
      {#each displays as d (d.id)}
        <li>
          {#if editingId === d.id}
            <form class="inline" onsubmit={(e) => saveRename(e, d.id)}>
              <input bind:value={editName} aria-label="New name" required />
              <button type="submit">Save</button>
              <button type="button" class="link" onclick={() => (editingId = null)}>Cancel</button>
            </form>
          {:else if scheduleId === d.id}
            <ScheduleEditor display={d} {dashboards} onclose={closeSchedule} onerror={handle} />
          {:else if pairingId === d.id}
            <form class="inline" onsubmit={(e) => pair(e, d.id)}>
              <input
                bind:value={pairCode}
                aria-label="Code shown on the screen"
                placeholder="ABCD-EFGH"
                autocomplete="off"
                spellcheck="false"
                required
              />
              <button type="submit">Pair</button>
              <button type="button" class="link" onclick={() => (pairingId = null)}>Cancel</button>
            </form>
          {:else}
            <div class="grow">
              <strong>{d.name}</strong>
              <span class="muted">{describeDevice(d.castDeviceId)} · {describeScreens(d)} · {describeHours(d)}</span>
              {#if d.cast}
                <span
                  class="badge"
                  class:ok={d.cast.state === 'showing'}
                  class:warn={d.cast.state === 'other_app'}
                  class:bad={d.cast.state === 'unreachable'}
                  title={d.cast.problem ?? ''}
                >
                  {describeCast(d.cast)}
                </span>
              {/if}
            </div>
            <select
              aria-label="Dashboard for {d.name}"
              value={d.dashboardId || 'default'}
              onchange={(e) => chooseDashboard(d, (e.target as HTMLSelectElement).value)}
            >
              {#each dashboards as b (b.id)}
                <option value={b.id}>{b.name}</option>
              {/each}
            </select>
            <select
              aria-label="Text size for {d.name}"
              value={d.textScale || 100}
              onchange={(e) => chooseTextScale(d, Number((e.target as HTMLSelectElement).value))}
            >
              {#each TEXT_SIZES as [value, label] (value)}
                <option {value}>{label}</option>
              {/each}
            </select>
            <a href="/d/{d.id}" target="_blank" rel="noopener">Open</a>
            <button class="link" onclick={() => (scheduleId = d.id)}>Schedule</button>
            <button class="link" onclick={() => startPairing(d)}>Pair a screen</button>
            {#if (d.pairedScreens ?? 0) > 0}
              <button class="link" onclick={() => unpair(d)}>Unpair screens</button>
            {/if}
            <button class="link" onclick={() => startRename(d)}>Rename</button>
            <button class="link danger" onclick={() => remove(d)}>Remove</button>
          {/if}
        </li>
      {/each}
    </ul>
    {#if rowError}<p class="error">{rowError}</p>{/if}
  {/if}
</section>

<AddDisplay {devices} {searching} {searchMessage} onsearch={search} oncreated={load} onerror={handle} />
