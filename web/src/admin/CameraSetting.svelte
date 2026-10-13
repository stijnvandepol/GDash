<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { api, type Camera, type CameraTest } from './api';
  import { describeSize, hostOf } from './customSources';
  import CameraForm from './CameraForm.svelte';

  // internal/camera allows this many cameras.
  const MAX_CAMERAS = 10;

  let {
    onchanged,
    onerror,
  }: {
    /** Called after a camera is added, changed or removed, so the card follows. */
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let cameras = $state<Camera[]>([]);
  let unreadable = $state(false);
  let error = $state('');
  // 'new', the id of the camera being edited, or null.
  let editing = $state<'new' | number | null>(null);
  let testing = $state<number | null>(null);
  // The last successful test, with a number that makes its preview load anew.
  let result = $state<{ id: number; test: CameraTest; shot: number } | null>(null);

  onMount(() => void load());

  async function load() {
    try {
      const list = await api.cameras();
      cameras = list.cameras;
      unreadable = list.unreadable;
    } catch (err) {
      error = onerror(err);
    }
  }

  async function saved(camera: Camera) {
    toast.success(`${camera.name} is saved.`);
    editing = null;
    if (result?.id === camera.id) result = null;
    await load();
    await onchanged();
  }

  async function remove(camera: Camera) {
    const message = 'Camera widgets that show it then say that their camera is not available. Its password or key is deleted.';
    if (!(await ask({ title: `Remove ${camera.name}?`, message, action: 'Remove' }))) return;
    error = '';
    try {
      await api.deleteCamera(camera.id);
      if (result?.id === camera.id) result = null;
      await load();
      await onchanged();
    } catch (err) {
      error = onerror(err);
    }
  }

  async function test(camera: Camera) {
    error = '';
    testing = camera.id;
    try {
      result = { id: camera.id, test: await api.testCamera(camera.id), shot: Date.now() };
    } catch (err) {
      result = null;
      error = onerror(err);
    } finally {
      testing = null;
    }
  }
</script>

<section class="card">
  <h2>Cameras</h2>
  <p class="muted">
    Show a still image of a camera at home in a Camera widget, refreshed every few seconds. GDash fetches the image
    itself, so the TVs never see the camera's address or password, and keeps only the latest image in memory. A
    camera is only asked while a screen shows it.
  </p>
  {#if unreadable}
    <p class="field-error">The cameras cannot be read on this server. Add them again.</p>
  {/if}
  {#if cameras.length > 0}
    <ul class="list">
      {#each cameras as camera (camera.id)}
        <li>
          <div class="grow">
            <strong>{camera.name}</strong>
            <span class="muted small">{hostOf(camera.url)}{camera.auth.kind === 'none' ? '' : ' · with authentication'}</span>
          </div>
          <button class="link" disabled={testing !== null} onclick={() => test(camera)}>
            {testing === camera.id ? 'Testing…' : 'Test'}
          </button>
          <button class="link" onclick={() => (editing = camera.id)}>Edit</button>
          <button class="link danger" onclick={() => remove(camera)}>Remove</button>
        </li>
        {#if editing === camera.id}
          <li class="form-row">
            <CameraForm {camera} onsaved={saved} oncancel={() => (editing = null)} {onerror} />
          </li>
        {/if}
        {#if result?.id === camera.id}
          <li class="form-row">
            <div class="result">
              <p class="small muted">The camera sent a {result.test.contentType.replace('image/', '').toUpperCase()} image of {describeSize(result.test.bytes)}.</p>
              <img src="/api/cameras/{camera.id}/image?shot={result.shot}" alt="The latest image of {camera.name}" />
              <button class="link" onclick={() => (result = null)}>Close</button>
            </div>
          </li>
        {/if}
      {/each}
    </ul>
  {/if}
  {#if editing === 'new'}
    <CameraForm onsaved={saved} oncancel={() => (editing = null)} {onerror} />
  {:else if cameras.length < MAX_CAMERAS}
    <div><button class="secondary" onclick={() => (editing = 'new')}>Add a camera</button></div>
  {:else}
    <p class="small muted">The limit of {MAX_CAMERAS} cameras is reached.</p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .form-row {
    display: block;
  }
  .result {
    display: grid;
    gap: var(--s-2);
    justify-items: start;
  }
  .result img {
    max-width: min(100%, 28rem);
    border-radius: var(--r-md);
  }
</style>
