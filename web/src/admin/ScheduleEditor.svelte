<script lang="ts">
  import { api, ApiError, type ActiveBlock, type Dashboard, type Display, type Slot } from './api';
  import { describeBlocks } from './displays';
  import Icon from './Icon.svelte';

  let {
    display,
    dashboards,
    onclose,
    onerror,
  }: {
    display: Display;
    dashboards: Dashboard[];
    /** Called after a save (saved = true) or a cancel. */
    onclose: (saved: boolean) => void;
    /** Turns an unexpected error into a message, or ends the session. */
    onerror: (err: unknown) => string;
  } = $props();

  // The server allows six slots (displays.MaxSlots) and 21 blocks (schedule.MaxBlocks).
  const maxSlots = 6;
  const maxBlocks = 21;

  // The day names the server uses, with their labels; Monday first.
  const DAYS = [
    ['mon', 'Mo'],
    ['tue', 'Tu'],
    ['wed', 'We'],
    ['thu', 'Th'],
    ['fri', 'Fr'],
    ['sat', 'Sa'],
    ['sun', 'Su'],
  ] as const;
  const ALL_DAYS: string[] = DAYS.map(([day]) => day);
  const PRESETS: [string, string[]][] = [
    ['Weekdays', ['mon', 'tue', 'wed', 'thu', 'fri']],
    ['Weekend', ['sat', 'sun']],
    ['Every day', ALL_DAYS],
  ];

  // The form edits a copy; the display prop only seeds it. A block that
  // the server stores without days (every day) shows all seven pressed.
  // svelte-ignore state_referenced_locally
  let blocks = $state<ActiveBlock[]>(display.active.map((b) => ({ ...b, days: b.days.length === 0 ? [...ALL_DAYS] : [...b.days] })));
  // svelte-ignore state_referenced_locally
  let slots = $state<Slot[]>(display.schedule.map((s) => ({ ...s, days: [...s.days] })));
  // svelte-ignore state_referenced_locally
  let rotation = $state<string[]>([...display.rotation]);
  // svelte-ignore state_referenced_locally
  let rotateSeconds = $state(display.rotateSeconds || 30);

  // The server allows five pages besides the display's own (displays.MaxRotation).
  const MAX_PAGES = 5;
  const ROTATE_CHOICES = [15, 30, 60, 120, 300];
  // The display's own dashboard, named in the explanations.
  const ownName = $derived.by(() => {
    const own = dashboards.find((b) => b.id === (display.dashboardId || 'default'));
    return own ? `“${own.name}”` : 'its own dashboard';
  });
  const others = $derived(dashboards.filter((b) => b.id !== (display.dashboardId || 'default')));

  function togglePage(id: string) {
    if (rotation.includes(id)) rotation = rotation.filter((r) => r !== id);
    else if (rotation.length < MAX_PAGES) rotation = [...rotation, id];
  }

  function describeSeconds(s: number): string {
    return s < 60 ? `${s} seconds` : s === 60 ? '1 minute' : `${s / 60} minutes`;
  }

  function toggleDay(days: string[], day: string): string[] {
    return days.includes(day) ? days.filter((d) => d !== day) : [...days, day];
  }
  function samePress(days: string[], preset: string[]): boolean {
    return days.length === preset.length && preset.every((d) => days.includes(d));
  }

  let error = $state('');
  let saving = $state(false);

  // A new block is the weekdays 08:00 to 22:00; the next ones start with
  // every day pressed, so the days to drop are one click each.
  function addBlock() {
    blocks.push({ days: blocks.length === 0 ? PRESETS[0]![1].slice() : ALL_DAYS.slice(), from: '08:00', until: '22:00' });
  }

  function addSlot() {
    const other = dashboards.find((b) => b.id !== (display.dashboardId || 'default')) ?? dashboards[0];
    slots.push({ from: '06:30', until: '09:00', days: [], dashboardId: other?.id ?? 'default' });
  }

  /** What the screen does, in words, as the blocks stand now. */
  const summary = $derived(blocks.length === 0 ? '' : describeBlocks(blocks));

  async function save(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    const empty = blocks.findIndex((b) => b.days.length === 0);
    if (empty >= 0) {
      error = `Time ${empty + 1}: choose at least one day.`;
      return;
    }
    saving = true;
    try {
      await api.updateDisplay(display.id, {
        // All seven days are stored as "every day", so a plan means the same
        // after a day is added to the week.
        active: blocks.map((b) => ({ days: b.days.length === ALL_DAYS.length ? [] : ALL_DAYS.filter((d) => b.days.includes(d)), from: b.from, until: b.until })),
        schedule: slots,
        rotation,
        rotateSeconds: rotation.length > 0 ? rotateSeconds : 0,
      });
      onclose(true);
    } catch (err) {
      error = err instanceof ApiError ? (fieldProblem(err) ?? err.message) : onerror(err);
    } finally {
      saving = false;
    }
  }

  function fieldProblem(err: ApiError): string | undefined {
    for (let i = 0; i < blocks.length; i++) {
      const msg = err.fieldMessage(`active[${i}]`);
      if (msg) return `Time ${i + 1} for the screen: ${msg}`;
    }
    const active = err.fieldMessage('active');
    if (active) return `When the screen is on: ${active}`;
    for (let i = 0; i < slots.length; i++) {
      const msg = err.fieldMessage(`schedule[${i}]`);
      if (msg) return `Other dashboard ${i + 1}: ${msg}`;
    }
    const pages = err.fieldMessage('rotation');
    if (pages) return `Pages in turn: ${pages}`;
    return err.fieldMessage('schedule');
  }
</script>

<form class="schedule" onsubmit={save}>
  <h3>Schedule for {display.name}</h3>

  <fieldset>
    <legend>When the screen is on</legend>
    {#if blocks.length === 0}
      <p>Always on, all day, every day.</p>
      <p class="muted small">Add a time to have the screen on only then. Outside the times it stays dark, so the TV can sleep.</p>
    {:else}
      <p class="muted small">
        The screen is on during these times, and dark in between so the TV can sleep. You can add several times on one
        day, for example a morning and an evening.
      </p>
    {/if}
    {#each blocks as block, i (i)}
      <div class="slot" role="group" aria-label="Time {i + 1} for the screen">
        <div class="days" role="group" aria-label="Days for time {i + 1}">
          {#each DAYS as [day, label] (day)}
            <button
              type="button"
              class="day"
              class:on={block.days.includes(day)}
              aria-pressed={block.days.includes(day)}
              onclick={() => (block.days = toggleDay(block.days, day))}>{label}</button
            >
          {/each}
          {#each PRESETS as [name, days] (name)}
            <button type="button" class="link small" aria-pressed={samePress(block.days, days)} onclick={() => (block.days = [...days])}>{name}</button>
          {/each}
        </div>
        <div class="inline">
          <label class="inline-time">From <input type="time" bind:value={block.from} required /></label>
          <label class="inline-time">until <input type="time" bind:value={block.until} required /></label>
          <button type="button" class="link danger" onclick={() => blocks.splice(i, 1)}>Remove</button>
        </div>
      </div>
    {/each}
    {#if summary}
      <p class="summary"><strong>On:</strong> {summary}. <span class="muted">Dark otherwise.</span></p>
    {/if}
    {#if blocks.length < maxBlocks}
      <div><button type="button" class="secondary" onclick={addBlock}>Add a time</button></div>
    {/if}
  </fieldset>

  <details class="optional" open={slots.length > 0}>
    <summary>Show another dashboard at set times</summary>
    <p class="muted small">
      This changes only what is shown, not whether the screen is on: use the times above for that. For example the
      school run on weekday mornings. Outside these times the display shows {ownName}. When times overlap, the first
      one wins.
    </p>
    {#each slots as slot, i (i)}
      <div class="slot">
        <div class="inline">
          <label class="inline-time">From <input type="time" bind:value={slot.from} required /></label>
          <label class="inline-time">until <input type="time" bind:value={slot.until} required /></label>
          <select aria-label="Dashboard for time slot {i + 1}" bind:value={slot.dashboardId}>
            {#each dashboards as b (b.id)}
              <option value={b.id}>{b.name}</option>
            {/each}
          </select>
          <button type="button" class="link danger" onclick={() => slots.splice(i, 1)}>Remove</button>
        </div>
        <div class="days" role="group" aria-label="Days for time slot {i + 1}">
          {#each DAYS as [day, label] (day)}
            <button
              type="button"
              class="day"
              class:on={slot.days.includes(day)}
              aria-pressed={slot.days.includes(day)}
              onclick={() => (slot.days = toggleDay(slot.days, day))}>{label}</button
            >
          {/each}
          <span class="muted small">{slot.days.length === 0 ? 'every day' : ''}</span>
        </div>
      </div>
    {:else}
      <p class="muted small">No set times yet.</p>
    {/each}
    {#if slots.length < maxSlots}
      <div><button type="button" class="secondary" onclick={addSlot}>Add a time</button></div>
    {/if}
  </details>

  {#if others.length > 0}
    <fieldset>
      <legend>Dashboards in turn</legend>
      <p class="muted small">Outside the set times, show these one after another with {ownName}.</p>
      <div class="days" role="group" aria-label="Dashboards shown in turn">
        {#each others as b (b.id)}
          <button
            type="button"
            class="day"
            class:on={rotation.includes(b.id)}
            aria-pressed={rotation.includes(b.id)}
            onclick={() => togglePage(b.id)}
            >{#if rotation.includes(b.id)}<Icon name="check" size={14} />{/if}{b.name}</button
          >
        {/each}
      </div>
      {#if rotation.length > 0}
        <label class="inline-time">
          Each page for
          <select bind:value={rotateSeconds}>
            {#each ROTATE_CHOICES as s (s)}
              <option value={s}>{describeSeconds(s)}</option>
            {/each}
          </select>
        </label>
      {/if}
    </fieldset>
  {/if}

  <div class="inline">
    <button type="submit" disabled={saving}>Save</button>
    <button type="button" class="link" onclick={() => onclose(false)}>Cancel</button>
  </div>
  {#if error}<p class="error">{error}</p>{/if}
</form>

<style>
  .schedule {
    display: grid;
    gap: var(--s-5);
    width: 100%;
  }
  .schedule p {
    margin: 0;
  }
  h3 {
    font-size: var(--fs-lg);
  }
  /* Each part of the schedule is ruled off, like the sections of a page. */
  fieldset {
    display: grid;
    gap: var(--s-2);
    margin: 0;
    padding: var(--s-3) 0 0;
    border: 0;
    border-top: 1px solid var(--border);
    min-width: 0;
  }
  legend {
    float: left;
    width: 100%;
    margin-bottom: var(--s-2);
    padding: 0;
    font-weight: 600;
  }
  .optional {
    display: grid;
    gap: var(--s-2);
    padding-top: var(--s-3);
    border-top: 1px solid var(--border);
  }
  .optional > summary {
    cursor: pointer;
    font-weight: 600;
  }
  .summary {
    padding: var(--s-2) var(--s-3);
    background: var(--surface);
    border-radius: var(--radius, 6px);
  }
  .slot {
    display: grid;
    gap: var(--s-2);
    padding-bottom: var(--s-3);
    border-bottom: 1px dashed var(--border);
  }
  .days {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    align-items: center;
  }
  /* A key that is pressed in or not, with its light on when it is. */
  .day {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-width: 2.6em;
    min-height: 34px;
    padding: 0 var(--s-3);
    background: var(--surface);
    color: var(--text-soft);
    border: 1px solid var(--border-strong);
    font-size: var(--fs-sm);
    transition: none;
  }
  .day:hover:not(:disabled) {
    background: var(--surface);
    border-color: var(--text);
    color: var(--text);
  }
  .day.on,
  .day.on:hover:not(:disabled) {
    background: var(--primary);
    color: var(--on-primary);
    border-color: var(--primary);
  }
  .day.on::before {
    content: '';
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--spark-on-primary);
  }
</style>
