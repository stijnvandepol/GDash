<script lang="ts">
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { onMount } from 'svelte';
  import { api, ApiError, type CastDevice, type Dashboard, type Display } from './api';
  import { errorMessage } from './errors';
  import { newestOnly } from './newest';
  import ScheduleEditor from './ScheduleEditor.svelte';
  import AddDisplay from './AddDisplay.svelte';
  import DisplayItem from './DisplayItem.svelte';
  import EmptyDisplays from './EmptyDisplays.svelte';

  let { onSessionEnded }: { onSessionEnded: () => void } = $props();

  const STATUS_REFRESH_MS = 15_000;

  let displays = $state<Display[]>([]);
  let dashboards = $state<Dashboard[]>([]);
  let devices = $state<CastDevice[]>([]);
  let searching = $state(false);
  let searchMessage = $state('');
  let loadError = $state('');
  // The TV grid, to draw each display's dashboard; fetched once.
  let grid = $state<{ columns: number; rows: number } | null>(null);

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
    api
      .widgetTypes()
      .then((c) => (grid = { columns: c.columns, rows: c.rows }))
      .catch(() => (grid = null));
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
    const message = `"${d.name}" stops showing GDash, and its paired screens are unpaired.`;
    if (!(await ask({ title: 'Remove this display?', message, action: 'Remove' }))) return;
    try {
      await api.deleteDisplay(d.id);
      toast.success(`Removed ${d.name}.`);
      await load();
    } catch (err) {
      rowError = handle(err);
    }
  }

  // Text sizes the server offers (internal/displays.TextScales).
  const TEXT_SIZES: [number, string][] = [
    [100, '100 % · a TV across the room'],
    [125, '125 %'],
    [150, '150 % · a smart display or tablet'],
    [200, '200 %'],
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
      toast.success('Screen paired. It shows the dashboard within seconds.');
      pairingId = null;
      await load();
    } catch (err) {
      rowError = handle(err);
    }
  }

  async function unpair(d: Display) {
    const message = `Every screen showing "${d.name}" will ask for a new code.`;
    if (!(await ask({ title: 'Unpair the screens?', message, action: 'Unpair' }))) return;
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

<header class="page-head">
  <div>
    <h1 class="page-title">Displays</h1>
    <p class="page-lede">
      {#if displays.length === 0}
        A display is a TV or a browser that shows one of your dashboards.
      {:else}
        {displays.length === 1 ? 'One TV or browser' : `${displays.length} TVs and browsers`}, each showing a dashboard.
      {/if}
    </p>
  </div>
  {#if displays.length > 0}
    <button class="secondary" onclick={() => document.getElementById('display-name')?.focus()}>Add a display</button>
  {/if}
</header>

<section class="card wide" aria-label="Your displays">
  {#if loadError}<p class="error">{loadError}</p>{/if}
  {#if displays.length === 0 && !loadError}
    <EmptyDisplays />
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
            <DisplayItem
              display={d}
              {dashboards}
              textSizes={TEXT_SIZES}
              {grid}
              meta="{describeDevice(d.castDeviceId)} · {describeScreens(d)} · {describeHours(d)}"
              ondashboard={(id) => chooseDashboard(d, id)}
              ontextscale={(percent) => chooseTextScale(d, percent)}
              onschedule={() => (scheduleId = d.id)}
              onpair={() => startPairing(d)}
              onunpair={() => unpair(d)}
              onrename={() => startRename(d)}
              onremove={() => remove(d)}
            />
          {/if}
        </li>
      {/each}
    </ul>
    {#if rowError}<p class="error">{rowError}</p>{/if}
  {/if}
</section>

<AddDisplay {devices} {searching} {searchMessage} onsearch={search} oncreated={load} onerror={handle} />
