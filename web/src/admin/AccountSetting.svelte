<script lang="ts">
  import { api, ApiError } from './api';

  let { onerror }: { onerror: (err: unknown) => string } = $props();

  let current = $state('');
  let next = $state('');
  let repeat = $state('');
  let fieldErrors = $state<{ current?: string; next?: string; repeat?: string }>({});
  let error = $state('');
  let done = $state(false);
  let busy = $state(false);

  async function change(event: SubmitEvent) {
    event.preventDefault();
    fieldErrors = {};
    error = '';
    done = false;
    if (next !== repeat) {
      fieldErrors = { repeat: 'the two new passwords differ' };
      return;
    }
    busy = true;
    try {
      await api.changePassword(current, next);
      current = next = repeat = '';
      done = true;
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = { current: err.fieldMessage('currentPassword'), next: err.fieldMessage('newPassword') };
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }
</script>

<section class="card">
  <h2>Password</h2>
  <form onsubmit={change}>
    <label>
      Current password
      <input type="password" bind:value={current} autocomplete="current-password" required />
      {#if fieldErrors.current}<span class="field-error">{fieldErrors.current}</span>{/if}
    </label>
    <label>
      New password
      <input type="password" bind:value={next} autocomplete="new-password" minlength="10" required />
      {#if fieldErrors.next}<span class="field-error">{fieldErrors.next}</span>{/if}
    </label>
    <label>
      New password again
      <input type="password" bind:value={repeat} autocomplete="new-password" required />
      {#if fieldErrors.repeat}<span class="field-error">{fieldErrors.repeat}</span>{/if}
    </label>
    <button type="submit" disabled={busy}>Change password</button>
  </form>
  {#if done}<p class="muted">Password changed. Other browsers that were logged in must log in again.</p>{/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>
