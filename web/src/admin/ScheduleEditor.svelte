<script lang="ts">
  import { api, ApiError, type Dashboard, type Display, type Slot } from './api';

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

  // The server allows six slots; displays.MaxSlots holds the same number.
  const maxSlots = 6;

  // The form edits a copy; the display prop only seeds it.
  // svelte-ignore state_referenced_locally
  let alwaysOn = $state(display.activeFrom === '');
  // svelte-ignore state_referenced_locally
  let from = $state(display.activeFrom || '06:30');
  // svelte-ignore state_referenced_locally
  let until = $state(display.activeUntil || '23:00');
  // svelte-ignore state_referenced_locally
  let slots = $state<Slot[]>(display.schedule.map((s) => ({ ...s })));
  let error = $state('');
  let saving = $state(false);

  function add() {
    const other = dashboards.find((b) => b.id !== (display.dashboardId || 'default')) ?? dashboards[0];
    slots.push({ from: '06:30', until: '09:00', dashboardId: other?.id ?? 'default' });
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    saving = true;
    error = '';
    try {
      await api.updateDisplay(display.id, {
        activeFrom: alwaysOn ? '' : from,
        activeUntil: alwaysOn ? '' : until,
        schedule: slots,
      });
      onclose(true);
    } catch (err) {
      error = err instanceof ApiError ? (fieldProblem(err) ?? err.message) : onerror(err);
    } finally {
      saving = false;
    }
  }

  function fieldProblem(err: ApiError): string | undefined {
    const hours = err.fieldMessage('activeHours');
    if (hours) return `Active hours: ${hours}`;
    for (let i = 0; i < slots.length; i++) {
      const msg = err.fieldMessage(`schedule[${i}]`);
      if (msg) return `Time slot ${i + 1}: ${msg}`;
    }
    return err.fieldMessage('schedule');
  }
</script>

<form class="schedule" onsubmit={save}>
  <div class="inline">
    <label class="inline-time"><input type="checkbox" bind:checked={alwaysOn} /> Always on</label>
    {#if !alwaysOn}
      <label class="inline-time">On from <input type="time" bind:value={from} required /></label>
      <label class="inline-time">until <input type="time" bind:value={until} required /></label>
    {/if}
  </div>
  <p class="muted">Outside its active hours the cast stops and the screen stays dark.</p>
  <p class="muted">
    Show another dashboard during part of the day. Outside these times the display shows its own dashboard. When
    slots overlap, the first one wins.
  </p>
  {#each slots as slot, i (i)}
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
  {:else}
    <p class="muted">No time slots: the display shows its own dashboard all day.</p>
  {/each}
  <div class="inline">
    {#if slots.length < maxSlots}
      <button type="button" class="link" onclick={add}>Add a time slot</button>
    {/if}
    <button type="submit" disabled={saving}>Save</button>
    <button type="button" class="link" onclick={() => onclose(false)}>Cancel</button>
  </div>
  {#if error}<p class="error">{error}</p>{/if}
</form>

<style>
  .schedule {
    display: grid;
    gap: 8px;
    width: 100%;
  }
  .schedule p {
    margin: 0;
  }
</style>
