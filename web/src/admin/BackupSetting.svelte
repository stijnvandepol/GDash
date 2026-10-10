<script lang="ts">
  import { api, ApiError } from './api';
  import { toast } from './toast.svelte';

  let { onerror }: { onerror: (err: unknown) => string } = $props();

  let password = $state('');
  let fieldError = $state('');
  let error = $state('');
  let busy = $state(false);

  async function download(event: SubmitEvent) {
    event.preventDefault();
    fieldError = '';
    error = '';
    busy = true;
    try {
      const file = await api.backup(password);
      password = '';
      // The answer is a file; a temporary link hands it to the browser. The
      // address is freed a moment later, as Firefox can still be reading it.
      const link = document.createElement('a');
      link.href = URL.createObjectURL(file);
      const today = new Date();
      today.setMinutes(today.getMinutes() - today.getTimezoneOffset());
      link.download = `gdash-backup-${today.toISOString().slice(0, 10)}.zip`;
      link.click();
      setTimeout(() => URL.revokeObjectURL(link.href), 10_000);
      toast.success('Backup downloaded. Keep it somewhere safe.');
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldError = err.fieldMessage('password') ?? '';
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }
</script>

<section class="card">
  <h2>Backup</h2>
  <p class="muted">
    One file with your displays, dashboards, settings and photos, to move GDash to another server or to keep
    somewhere safe. It also holds the key that unlocks your calendar addresses and the Home Assistant token, so
    treat it like a password. GDash also keeps a copy of its database every day in the data directory.
  </p>
  <form onsubmit={download}>
    <label>
      Your password
      <input type="password" bind:value={password} autocomplete="current-password" required />
      {#if fieldError}<span class="field-error">{fieldError}</span>{/if}
    </label>
    <button type="submit" class="secondary" class:busy disabled={busy}>
      {busy ? 'Preparing…' : 'Download backup'}
    </button>
  </form>
  {#if error}<p class="error">{error}</p>{/if}
</section>
