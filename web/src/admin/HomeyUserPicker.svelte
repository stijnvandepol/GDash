<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type HomeyUser } from './api';
  import { initials, MAX_BADGE_LABEL, type PresenceUser } from './homey';

  let {
    selected,
    max,
    onchange,
  }: {
    /** The people the widget looks for, in the order of their badges. */
    selected: PresenceUser[];
    max: number;
    onchange: (users: PresenceUser[]) => void;
  } = $props();

  let users = $state<HomeyUser[]>([]);
  let loadError = $state('');

  const names = $derived(new Map(users.map((u) => [u.id, u.name])));
  const others = $derived(users.filter((u) => !selected.some((s) => s.id === u.id)));

  onMount(async () => {
    try {
      users = await api.homeyUsers();
    } catch (err) {
      loadError = err instanceof Error ? err.message : String(err);
    }
  });

  function add(user: HomeyUser) {
    if (selected.length < max) onchange([...selected, { id: user.id, label: initials(user.name) }]);
  }

  function relabel(id: string, label: string) {
    onchange(selected.map((s) => (s.id === id ? { id, label } : s)));
  }
</script>

<div class="picker">
  <span>People</span>
  <p class="muted small">
    A badge with the label shows while that person is home in Homey. Screens are never told who is away.
  </p>
  {#if selected.length > 0}
    <ul>
      {#each selected as user (user.id)}
        {@const name = names.get(user.id) ?? 'Unknown user'}
        <li>
          <span class="grow">{name}</span>
          <label class="inline-label">
            Label
            <input
              class="label"
              value={user.label}
              maxlength={MAX_BADGE_LABEL}
              aria-label="Label for {name}"
              oninput={(e) => relabel(user.id, (e.target as HTMLInputElement).value)}
            />
          </label>
          <button type="button" class="link danger" onclick={() => onchange(selected.filter((s) => s.id !== user.id))}>
            Remove
          </button>
        </li>
      {/each}
    </ul>
  {:else}
    <p class="field-error">Choose at least one person.</p>
  {/if}
  {#if loadError}
    <p class="error">{loadError}</p>
  {:else if selected.length < max}
    {#if others.length > 0}
      <ul>
        {#each others as user (user.id)}
          <li>
            <button type="button" class="link" onclick={() => add(user)}>Add {user.name}</button>
          </li>
        {/each}
      </ul>
    {/if}
  {:else}
    <p class="muted small">This widget looks for at most {max} people.</p>
  {/if}
</div>

<style>
  .picker {
    display: grid;
    gap: 6px;
    min-width: 0;
  }
  p {
    margin: 0;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 4px;
  }
  li {
    display: flex;
    gap: 8px;
    align-items: center;
    flex-wrap: wrap;
    overflow-wrap: anywhere;
  }
  .grow {
    flex: 1 1 auto;
    min-width: 0;
  }
  .inline-label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 400;
  }
  .label {
    width: 7em;
  }
</style>
