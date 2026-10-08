<script lang="ts">
  import { api, ApiError, type User } from './api';

  let { onDone }: { onDone: (user: User) => void } = $props();

  let setupToken = $state('');
  let username = $state('');
  let password = $state('');
  let error = $state<ApiError | null>(null);
  let busy = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = null;
    try {
      onDone(await api.setup(setupToken.trim(), username, password));
    } catch (err) {
      error = err instanceof ApiError ? err : new ApiError(0, 'network', 'De server is niet bereikbaar.');
    } finally {
      busy = false;
    }
  }
</script>

<form class="card narrow" onsubmit={submit}>
  <h1>Daycast instellen</h1>
  <p class="muted">
    Maak het beheerdersaccount aan. De setup-token staat in de log van Daycast, bijvoorbeeld in Container Manager op je NAS
    of in het terminalvenster. Zo kan alleen jij deze server instellen.
  </p>

  <label>
    Setup-token
    <input bind:value={setupToken} autocomplete="off" spellcheck="false" required />
  </label>
  <label>
    Gebruikersnaam
    <input bind:value={username} autocomplete="username" required />
    {#if error?.fieldMessage('username')}<span class="field-error">{error.fieldMessage('username')}</span>{/if}
  </label>
  <label>
    Wachtwoord
    <input type="password" bind:value={password} autocomplete="new-password" required minlength="10" />
    {#if error?.fieldMessage('password')}<span class="field-error">{error.fieldMessage('password')}</span>{/if}
  </label>

  {#if error && error.fields.length === 0}<p class="error">{error.message}</p>{/if}
  <button type="submit" disabled={busy}>{busy ? 'Bezig…' : 'Account aanmaken'}</button>
</form>
