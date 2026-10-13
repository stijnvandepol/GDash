<script lang="ts">
  import { api, ApiError, type Camera, type CameraInput } from './api';
  import { AUTH_KINDS, secretPlaceholder } from './customSources';

  let {
    camera,
    onsaved,
    oncancel,
    onerror,
  }: {
    /** The camera to change, or undefined for a new one. */
    camera?: Camera;
    onsaved: (saved: Camera) => Promise<void>;
    oncancel: () => void;
    onerror: (err: unknown) => string;
  } = $props();

  // The form starts from the camera once; a reload of the list must not
  // overwrite what is being typed. Secrets stay empty, which keeps them.
  // svelte-ignore state_referenced_locally
  let form = $state<CameraInput>(
    camera
      ? { name: camera.name, url: camera.url, auth: { kind: camera.auth.kind, name: camera.auth.name, secret: '' } }
      : { name: '', url: '', auth: { kind: 'none', name: '', secret: '' } },
  );
  let fieldErrors = $state<Record<string, string>>({});
  let error = $state('');
  let busy = $state(false);

  const auth = $derived(AUTH_KINDS.find((a) => a.kind === form.auth.kind) ?? AUTH_KINDS[0]!);

  async function save(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    fieldErrors = {};
    busy = true;
    try {
      await onsaved(camera ? await api.saveCamera(camera.id, form) : await api.addCamera(form));
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = Object.fromEntries(err.fields.map((f) => [f.field, f.message]));
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={save} class="camera-form">
  <label>
    Name
    <input bind:value={form.name} maxlength="60" required placeholder="Front door" />
    {#if fieldErrors.name}<span class="field-error">{fieldErrors.name}</span>{/if}
  </label>
  <label>
    Snapshot address
    <input
      bind:value={form.url}
      type="url"
      required
      autocomplete="off"
      spellcheck="false"
      placeholder="http://192.168.1.20:5000/api/front_door/latest.jpg"
    />
    <span class="small muted">
      The address of one JPEG, PNG or WebP image, not a video stream: Frigate's /api/&lt;camera&gt;/latest.jpg, a Reolink or
      Hikvision snapshot address, or Home Assistant's /api/camera_proxy/camera.&lt;name&gt;.
    </span>
    {#if fieldErrors.url}<span class="field-error">{fieldErrors.url}</span>{/if}
  </label>
  <label>
    Authentication
    <select bind:value={form.auth.kind}>
      <!-- A camera has no session to log in for; only a custom API does. -->
      {#each AUTH_KINDS.filter((a) => a.kind !== 'login') as a (a.kind)}<option value={a.kind}>{a.kind === 'none' ? 'None' : a.label}</option>{/each}
    </select>
    {#if fieldErrors['auth.kind']}<span class="field-error">{fieldErrors['auth.kind']}</span>{/if}
  </label>
  {#if auth.nameLabel}
    <label>
      {auth.nameLabel}
      <input bind:value={form.auth.name} autocomplete="off" spellcheck="false" />
      {#if fieldErrors['auth.name']}<span class="field-error">{fieldErrors['auth.name']}</span>{/if}
    </label>
  {/if}
  {#if auth.secretLabel}
    <label>
      {auth.secretLabel}
      <!-- Not type="password", so browsers do not offer to save it. -->
      <input
        bind:value={form.auth.secret}
        class="masked"
        autocomplete="off"
        spellcheck="false"
        placeholder={secretPlaceholder(camera, form.auth.kind)}
      />
      <span class="small muted">Stored encrypted and never shown again.</span>
      {#if fieldErrors['auth.secret']}<span class="field-error">{fieldErrors['auth.secret']}</span>{/if}
    </label>
  {/if}
  <div class="inline">
    <button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
    <button type="button" class="link" onclick={oncancel}>Cancel</button>
  </div>
  {#if error}<p class="error">{error}</p>{/if}
</form>

<style>
  .camera-form {
    display: grid;
    gap: var(--s-3);
    max-width: 40rem;
  }
  .masked {
    -webkit-text-security: disc;
  }
</style>
