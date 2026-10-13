<script lang="ts">
  import { api, ApiError, type CustomSource } from './api';
  import { AUTH_KINDS, INTERVALS } from './customSources';
  import PresetAnswer from './PresetAnswer.svelte';
  import { answersFrom, placeholderProblem, presetInput, type Preset } from './presets';

  // Setting up one device or service of a preset, or changing one: only
  // what is the household's own, such as an IP address or a token. How
  // GDash asks it is under Advanced. Test asks it without saving.
  let {
    preset,
    source,
    defaultName,
    onsaved,
    oncancel,
    onerror,
  }: {
    preset: Preset;
    /** The setup being changed, or undefined for a new one. */
    source?: CustomSource;
    /** The name of a new setup. */
    defaultName: string;
    onsaved: (source: CustomSource) => void;
    /** Absent when there is nothing to go back to. */
    oncancel?: () => void;
    onerror: (err: unknown) => string;
  } = $props();

  // svelte-ignore state_referenced_locally
  let name = $state(source?.name ?? defaultName);
  // svelte-ignore state_referenced_locally
  let answers = $state<Record<string, string>>((source && answersFrom(preset, source.url)) ?? {});
  let secret = $state('');
  // svelte-ignore state_referenced_locally
  let user = $state(source && preset.source.auth.kind === 'basic' ? source.auth.name : '');
  // svelte-ignore state_referenced_locally
  let intervalSeconds = $state(source?.intervalSeconds ?? preset.source.intervalSeconds);
  let problems = $state<Record<string, string>>({});
  let error = $state('');
  let busy = $state<'' | 'testing' | 'saving'>('');
  let advanced = $state(false);
  // The answer of Test: the data, why it failed, or null while asking.
  let answer = $state<{ data: unknown } | { problem: string } | null | undefined>(undefined);

  const auth = $derived(AUTH_KINDS.find((a) => a.kind === preset.source.auth.kind));
  const ids = $derived(`preset-${preset.id}-${source?.id ?? 'new'}`);
  // A stored secret is kept when the field stays empty.
  const keepsSecret = $derived(source?.auth.secretSet === true && !source.unreadable);
  const authLabel = $derived(preset.source.auth.kind === 'login' ? 'Logs in first with the password' : (auth?.label ?? ''));

  function check(): boolean {
    const found: Record<string, string> = {};
    for (const p of preset.placeholders) {
      const problem = placeholderProblem(p.kind, answers[p.key] ?? '');
      if (problem) found[p.key] = problem;
    }
    if (name.trim() === '') found.name = 'Fill this in.';
    if (auth?.secretLabel && secret.trim() === '' && !keepsSecret) found.secret = 'Fill this in.';
    if (preset.source.auth.kind === 'basic' && user.trim() === '') found.user = 'Fill this in.';
    problems = found;
    if (found.name) advanced = true;
    return Object.keys(found).length === 0;
  }

  const input = () => presetInput(preset, { name, answers, secret, user, intervalSeconds });

  async function test() {
    error = '';
    if (!check()) return;
    busy = 'testing';
    answer = null;
    try {
      answer = { data: (await api.tryCustomSource(input(), source?.id)).data };
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) {
        answer = undefined;
        showFields(err);
      } else {
        answer = { problem: err instanceof ApiError && err.status !== 401 ? err.message : onerror(err) };
      }
    } finally {
      busy = '';
    }
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    if (!check()) return;
    busy = 'saving';
    try {
      let saved: CustomSource;
      if (source) {
        saved = await api.saveCustomSource(source.id, input());
      } else {
        const added = await api.addCustomSource(input());
        saved = await api.saveCustomSourceValues(added.id, { values: preset.values, lists: preset.lists });
      }
      secret = '';
      onsaved(saved);
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) showFields(err);
      else error = onerror(err);
    } finally {
      busy = '';
    }
  }

  function showFields(err: ApiError) {
    const fields = Object.fromEntries(err.fields.map((f) => [f.field, f.message]));
    problems = { name: fields.name ?? '', secret: fields['auth.secret'] ?? '', user: fields['auth.name'] ?? '' };
    if (problems.name) advanced = true;
    const rest = err.fields.filter((f) => !['name', 'auth.secret', 'auth.name'].includes(f.field));
    error = rest.map((f) => f.message).join('; ');
  }
</script>

<form class="preset-form" onsubmit={save} novalidate aria-label={source ? `Change ${source.name}` : `Set up ${preset.name}`}>
  {#each preset.placeholders as p (p.key)}
    <label>
      {p.label}
      <input
        value={answers[p.key] ?? ''}
        oninput={(e) => (answers[p.key] = (e.target as HTMLInputElement).value)}
        placeholder={p.example}
        autocomplete="off"
        spellcheck="false"
        inputmode={p.kind === 'origin' ? 'url' : undefined}
        aria-invalid={problems[p.key] ? 'true' : undefined}
        aria-describedby={problems[p.key] ? `${ids}-${p.key}-problem` : undefined}
      />
      {#if p.hint}<span class="small muted">{p.hint}</span>{/if}
      {#if problems[p.key]}<span class="field-error" id="{ids}-{p.key}-problem">{problems[p.key]}</span>{/if}
    </label>
  {/each}
  {#if preset.source.auth.kind === 'basic'}
    <label>
      {auth?.nameLabel}
      <input bind:value={user} autocomplete="off" spellcheck="false" aria-invalid={problems.user ? 'true' : undefined} aria-describedby={problems.user ? `${ids}-user-problem` : undefined} />
      {#if problems.user}<span class="field-error" id="{ids}-user-problem">{problems.user}</span>{/if}
    </label>
  {/if}
  {#if auth?.secretLabel}
    <label>
      {auth.secretLabel}
      <!-- Not type="password", so browsers do not offer to save it. -->
      <input
        bind:value={secret}
        class="masked"
        autocomplete="off"
        spellcheck="false"
        placeholder={keepsSecret ? 'Stored; leave empty to keep it' : ''}
        aria-invalid={problems.secret ? 'true' : undefined}
        aria-describedby={problems.secret ? `${ids}-secret-problem` : undefined}
      />
      <span class="small muted">{preset.source.secretHint ? `${preset.source.secretHint} ` : ''}Stored encrypted and never shown again.</span>
      {#if problems.secret}<span class="field-error" id="{ids}-secret-problem">{problems.secret}</span>{/if}
    </label>
  {/if}

  <details bind:open={advanced}>
    <summary>Advanced</summary>
    <div class="advanced">
      <label>
        Name
        <input bind:value={name} maxlength="60" autocomplete="off" aria-invalid={problems.name ? 'true' : undefined} aria-describedby="{ids}-name-hint" />
        <span class="small muted" id="{ids}-name-hint">{problems.name || 'Tells this one apart from others in the dashboard editor.'}</span>
      </label>
      <label>
        Refresh
        <select bind:value={intervalSeconds}>
          {#each INTERVALS as i (i.seconds)}<option value={i.seconds}>{i.label}</option>{/each}
        </select>
      </label>
      <dl class="small">
        <dt>Address</dt>
        <dd><code>{preset.source.method} {preset.source.url}</code></dd>
        <dt>Sign-in</dt>
        <dd>{authLabel}</dd>
        <dt>Reads</dt>
        <dd>{[...preset.values.map((v) => v.name), ...preset.lists.map((l) => l.name)].join(', ')}</dd>
      </dl>
      <p class="small muted">
        GDash asks it as a custom API. <a href={preset.docs} rel="noopener" target="_blank">The documentation it follows</a>.
      </p>
    </div>
  </details>

  {#if answer !== undefined}<PresetAnswer {preset} {answer} />{/if}

  <div class="inline">
    <button type="button" class="secondary" disabled={busy !== ''} onclick={test}>{busy === 'testing' ? 'Testing…' : 'Test'}</button>
    <button type="submit" disabled={busy !== ''}>{busy === 'saving' ? 'Saving…' : 'Save'}</button>
    {#if oncancel}<button type="button" class="link" onclick={oncancel}>Cancel</button>{/if}
  </div>
  {#if error}<p class="error">{error}</p>{/if}
</form>

<style>
  .preset-form {
    display: grid;
    gap: var(--s-3);
    min-width: 0;
  }
  .masked {
    -webkit-text-security: disc;
  }
  summary {
    cursor: pointer;
    color: var(--muted);
    font-size: var(--fs-sm);
    min-height: 32px;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  /* A flex summary loses the browser's triangle, so it draws its own. */
  summary::before {
    content: '▸';
  }
  details[open] > summary::before {
    content: '▾';
  }
  .advanced {
    display: grid;
    gap: var(--s-3);
    padding-top: var(--s-1);
  }
  dl {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: var(--s-1) var(--s-3);
    margin: 0;
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
  code {
    font-family: var(--mono);
  }
</style>
