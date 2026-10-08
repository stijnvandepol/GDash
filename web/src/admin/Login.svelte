<script lang="ts">
  import { api, ApiError, type User } from './api';

  let { onDone }: { onDone: (user: User) => void } = $props();

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = '';
    try {
      onDone(await api.login(username, password));
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'De server is niet bereikbaar.';
    } finally {
      busy = false;
    }
  }
</script>

<form class="card narrow" onsubmit={submit}>
  <h1>Inloggen</h1>
  <label>
    Gebruikersnaam
    <input bind:value={username} autocomplete="username" required />
  </label>
  <label>
    Wachtwoord
    <input type="password" bind:value={password} autocomplete="current-password" required />
  </label>
  {#if error}<p class="error">{error}</p>{/if}
  <button type="submit" disabled={busy}>{busy ? 'Bezig…' : 'Inloggen'}</button>
</form>
