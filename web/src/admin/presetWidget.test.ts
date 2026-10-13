import { describe, expect, it } from 'vitest';
import type { CustomSource } from './api';
import { PRESETS } from './presets';
import { newPresetWidget, presetPaletteEntries, presetWidget, togglePair, viewOf, viewSettings } from './presetWidget';
import { paletteGroups } from './widgetSettings';

const preset = (id: string) => PRESETS.find((p) => p.id === id)!;
const p1 = preset('homewizard-p1');
const piHole = preset('pi-hole');
const todoist = preset('todoist');

function source(id: number, name: string, presetId = ''): CustomSource {
  return {
    id,
    name,
    mode: 'fetch',
    url: 'http://192.168.1.50/api/v1/data',
    method: 'GET',
    body: '',
    auth: { kind: 'none', name: '', secretSet: false },
    headers: [],
    intervalSeconds: 30,
    sample: null,
    values: [],
    lists: [],
    preset: presetId,
    unreadable: false,
  };
}

const meter = source(4, 'P1 meter', 'homewizard-p1');
const garage = source(7, 'Garage meter', 'homewizard-p1');
const own = source(2, 'My API');

describe('viewSettings', () => {
  it('makes the P1 meter’s power now: the value with its unit and the suggested rules', () => {
    expect(viewSettings(p1, p1.views[0]!, 4)).toEqual({
      source: 4,
      mode: 'value',
      title: 'Power now',
      path: 'active_power_w',
      unit: 'W',
      decimals: 0,
      rules: [
        { when: 'number(value) < 0', tone: 'ok', message: 'Exporting to the grid' },
        { when: 'number(value) > 3000', tone: 'warn', message: 'Using a lot of power' },
      ],
    });
  });

  it('makes labelled values from the names, with a label of the view’s own where it has one', () => {
    expect(viewSettings(p1, p1.views[1]!, 4)).toEqual({
      source: 4,
      mode: 'keyValues',
      title: 'Meter readings',
      pairs: [
        { label: 'Electricity in', path: 'total_power_import_kwh' },
        { label: 'Electricity out', path: 'total_power_export_kwh' },
        { label: 'Gas', path: 'total_gas_m3' },
      ],
    });
    expect(viewSettings(piHole, piHole.views[0]!, 9).pairs).toEqual([
      { label: 'Blocked', path: 'queries.blocked' },
      { label: 'Share', path: 'queries.percent_blocked' },
    ]);
  });

  it('makes a list with every column of the named list', () => {
    expect(viewSettings(todoist, todoist.views[0]!, 3)).toEqual({
      source: 3,
      mode: 'list',
      title: 'Today',
      path: 'results',
      fields: ['content', 'due.date'],
      rules: [{ when: 'row.priority == 4', tone: 'warn' }],
    });
  });

  it('copies the rules, so changing a widget’s rules leaves the preset alone', () => {
    const settings = viewSettings(p1, p1.views[0]!, 4);
    (settings.rules as { tone: string }[])[0]!.tone = 'bad';
    expect(p1.views[0]!.rules![0]!.tone).toBe('ok');
  });
});

describe('viewOf', () => {
  it('finds the view a widget shows, also after its title or values changed', () => {
    expect(viewOf(p1, viewSettings(p1, p1.views[0]!, 4))?.id).toBe('power');
    expect(viewOf(p1, { ...viewSettings(p1, p1.views[1]!, 4), title: 'Mine', pairs: [] })?.id).toBe('readings');
    expect(viewOf(piHole, viewSettings(piHole, piHole.views[2]!, 4))?.id).toBe('blocked');
    expect(viewOf(piHole, viewSettings(piHole, piHole.views[1]!, 4))?.id).toBe('share');
  });

  it('gives the first view of the mode for a value of no view, and none for another mode', () => {
    expect(viewOf(p1, { mode: 'value', path: 'total_gas_m3' })?.id).toBe('power');
    expect(viewOf(p1, { mode: 'list', path: 'x' })).toBeUndefined();
  });
});

describe('presetWidget', () => {
  it('knows a Custom widget of a preset by its source, with every setup of the preset', () => {
    const sources = [own, meter, garage];
    const found = presetWidget({ source: 7, mode: 'value' }, sources);
    expect(found?.preset).toBe(p1);
    expect(found?.sources).toEqual([meter, garage]);
    expect(presetWidget({ source: 2, mode: 'value' }, sources)).toBeUndefined();
    expect(presetWidget({ source: 99, mode: 'value' }, sources)).toBeUndefined();
    expect(presetWidget({ source: 1, mode: 'value' }, [source(1, 'X', 'a-preset-from-elsewhere')])).toBeUndefined();
  });
});

describe('newPresetWidget', () => {
  it('starts as the first view of the first setup, at least as large as the type allows', () => {
    const made = newPresetWidget(p1, [own, meter, garage], { w: 4, h: 3 });
    expect(made).toEqual({ settings: viewSettings(p1, p1.views[0]!, 4), w: 5, h: 4 });
    expect(newPresetWidget(p1, [meter], { w: 6, h: 5 })).toMatchObject({ w: 6, h: 5 });
    expect(newPresetWidget(p1, [own], { w: 4, h: 3 })).toBeUndefined();
  });
});

describe('the palette', () => {
  const types = ['clock', 'custom', 'energyprice', 'calendar', 'news'];

  it('offers each preset that is set up once, by its own name, in its category', () => {
    const entries = presetPaletteEntries([own, meter, garage, source(8, 'Tasks', 'todoist'), source(9, 'DNS', 'pi-hole')]);
    expect(entries.map((e) => [e.label, e.category])).toEqual([
      ['HomeWizard P1 meter', 'home'],
      ['Pi-hole', 'home'],
      ['Todoist', 'agenda'],
    ]);
    expect(entries[0]).toMatchObject({ key: 'preset-homewizard-p1', type: 'custom', preset: 'homewizard-p1', icon: 'meter' });
    // One short line on what it can show: its views.
    expect(entries.map((e) => e.description)).toEqual(['Power now or meter readings', 'Blocked, blocked share or overview', 'Tasks']);

    const groups = paletteGroups(types, '', entries).map((g) => [g.label, g.items.map((i) => i.label)]);
    expect(groups).toEqual([
      ['Time & date', ['Clock']],
      ['Home', ['Energy price', 'Custom API', 'HomeWizard P1 meter', 'Pi-hole']],
      ['Agenda & household', ['Calendar', 'Todoist']],
      ['Media & news', ['News']],
    ]);
  });

  it('offers no preset without a setup, and none on a server without the Custom widget', () => {
    expect(presetPaletteEntries([own])).toEqual([]);
    const entries = presetPaletteEntries([meter]);
    expect(paletteGroups(['clock'], '', entries).flatMap((g) => g.items.map((i) => i.label))).toEqual(['Clock']);
  });

  it('finds a preset’s widget by its name, and the Custom API as your own JSON API', () => {
    const entries = presetPaletteEntries([meter]);
    expect(paletteGroups(types, 'homewizard', entries).flatMap((g) => g.items.map((i) => i.label))).toEqual(['HomeWizard P1 meter']);
    expect(paletteGroups(types, 'own json', entries).flatMap((g) => g.items.map((i) => i.label))).toEqual(['Custom API']);
  });
});

describe('togglePair', () => {
  const settings = viewSettings(piHole, piHole.views[0]!, 9);
  const named = (name: string) => piHole.values.find((v) => v.name === name)!;

  it('turns a value on in the preset’s order, labelled with its name, keeping the labels there are', () => {
    expect(togglePair(piHole, settings.pairs, named('Queries'), true)).toEqual([
      { label: 'Queries', path: 'queries.total' },
      { label: 'Blocked', path: 'queries.blocked' },
      { label: 'Share', path: 'queries.percent_blocked' },
    ]);
  });

  it('turns a value off and keeps a pair of another path at the end', () => {
    const imported = [...(settings.pairs as object[]), { label: 'Took', path: 'took' }];
    expect(togglePair(piHole, imported, named('Blocked'), false)).toEqual([
      { label: 'Share', path: 'queries.percent_blocked' },
      { label: 'Took', path: 'took' },
    ]);
  });
});
