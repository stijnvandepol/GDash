import type { Component } from 'svelte';
import { api } from './api';
import { describeSnapshot, isSnapshotLate } from './homey';
import { isPushLate } from './customSources';
import {
  NOT_SET_UP,
  builtinState,
  count,
  sourceState,
  type CatalogueEntry,
  type IntegrationContext,
  type IntegrationState,
} from './integrations';
import { PRESETS, sourcesFrom, type Preset } from './presets';
import ApiTokenSetting from './ApiTokenSetting.svelte';
import CameraSetting from './CameraSetting.svelte';
import CustomSourceSetting from './CustomSourceSetting.svelte';
import HomeAssistantSetting from './HomeAssistantSetting.svelte';
import HomeySetting from './HomeySetting.svelte';
import ImmichSetting from './ImmichSetting.svelte';
import LocationSetting from './LocationSetting.svelte';
import PhotoSetting from './PhotoSetting.svelte';
import PresetSetup from './PresetSetup.svelte';
import SourceList from './SourceList.svelte';
import SynologySetting from './SynologySetting.svelte';

// Every card on the Integrations page. Adding an integration is adding an
// entry here: what it is, how its state reads, and the setup it opens.

/** What a setup panel gets from the page. */
export interface PanelContext extends IntegrationContext {
  /** Reloads the settings and the cards' state after a change. */
  onchanged: () => Promise<void>;
  onerror: (err: unknown) => string;
  /** Opens another integration's setup, such as API tokens. */
  onopen: (id: string) => void;
}

/** A setup component with its props, ready to render. */
export interface PanelView {
  component: Component<any>;
  props: Record<string, unknown>;
}

function view<P extends Record<string, any>>(component: Component<P>, props: P): PanelView {
  return { component: component as Component<any>, props };
}

export interface Integration extends CatalogueEntry {
  state: (ctx: IntegrationContext) => IntegrationState;
  /** The setup it opens; a built-in source has none. */
  panel?: (ctx: PanelContext) => PanelView;
}

// The server allows these numbers; internal/settings holds the same.
const MAX_NEWS_FEEDS = 5;
const MAX_CALENDARS = 10;

const NEEDS_LOCATION: IntegrationState = { kind: 'idle', detail: 'Built in · needs the weather location' };
const NEEDS_CALENDAR: IntegrationState = { kind: 'idle', detail: 'Built in · needs a calendar' };
const NOT_FETCHED = 'Fetched once a dashboard shows it.';

const SERVICES: Integration[] = [
  {
    id: 'weather',
    name: 'Weather',
    description: 'Weather now and the forecast for your place, from Open-Meteo',
    keywords: 'open-meteo location forecast temperature',
    category: 'weather',
    icon: 'weather',
    tags: ['no-key'],
    kind: 'service',
    state: ({ settings, status }) => {
      if (!settings.location) return NOT_SET_UP;
      const st = sourceState(status['weather.current'], NOT_FETCHED);
      return { ...st, detail: `${settings.location.name}. ${st.detail}` };
    },
    panel: (ctx) =>
      view(LocationSetting, { location: ctx.settings.location, status: ctx.status['weather.current'], onchanged: ctx.onchanged, onerror: ctx.onerror }),
  },
  {
    id: 'news',
    name: 'News feeds',
    description: 'Headlines from RSS or Atom feeds, such as NOS',
    keywords: 'rss atom nos headlines',
    category: 'news',
    icon: 'news',
    tags: ['no-key'],
    kind: 'service',
    state: ({ settings, status }) => {
      const feeds = settings.newsFeeds.length;
      if (feeds === 0) return NOT_SET_UP;
      const st = sourceState(status['news.default'], NOT_FETCHED);
      return { ...st, detail: `${count(feeds, 'feed', 'feeds')}. ${st.detail}` };
    },
    panel: (ctx) =>
      view(SourceList, {
        title: 'News',
        intro: 'RSS or Atom feeds. With more than one, the newest headlines of all feeds are shown together.',
        items: ctx.settings.newsFeeds.map((f) => ({ id: f.id, name: f.name, detail: f.url })),
        status: ctx.status['news.default'],
        max: MAX_NEWS_FEEDS,
        namePlaceholder: 'NOS',
        urlPlaceholder: 'https://feeds.nos.nl/nosnieuwsalgemeen',
        onadd: async (name, url) => {
          await api.addNewsFeed(name, url);
          await ctx.onchanged();
        },
        onremove: async (id) => {
          await api.deleteNewsFeed(id);
          await ctx.onchanged();
        },
        onerror: ctx.onerror,
      }),
  },
  {
    id: 'calendars',
    name: 'Calendars',
    description: 'Appointments from Google, Outlook or iCloud by their iCal address',
    keywords: 'ical ics google outlook icloud agenda',
    category: 'lists',
    icon: 'calendar',
    tags: ['no-key'],
    kind: 'service',
    state: ({ settings, status }) => {
      const calendars = settings.calendars.length;
      if (calendars === 0) return NOT_SET_UP;
      if (settings.calendars.some((c) => c.unreadable)) return { kind: 'problem', detail: 'A calendar cannot be read on this server. Add it again.' };
      const st = sourceState(status['calendar.default'], NOT_FETCHED);
      return { ...st, detail: `${count(calendars, 'calendar', 'calendars')}. ${st.detail}` };
    },
    panel: (ctx) =>
      view(SourceList, {
        title: 'Calendars',
        intro:
          'Paste the secret iCal address of a Google, Outlook or iCloud calendar. Anyone with it can read the calendar, so GDash stores it encrypted and never shows it again.',
        items: ctx.settings.calendars.map((c) => ({
          id: c.id,
          name: c.name,
          detail: c.host,
          warning: c.unreadable ? 'This address cannot be read on this server. Remove it and add it again.' : undefined,
          color: c.color,
        })),
        status: ctx.status['calendar.default'],
        max: MAX_CALENDARS,
        secret: true,
        namePlaceholder: 'Family',
        urlPlaceholder: 'https://calendar.google.com/calendar/ical/…/basic.ics',
        onadd: async (name, url) => {
          await api.addCalendar(name, url);
          await ctx.onchanged();
        },
        onremove: async (id) => {
          await api.deleteCalendar(id);
          await ctx.onchanged();
        },
        oncolor: async (id, color) => {
          await api.setCalendarColor(id, color);
          await ctx.onchanged();
        },
        onerror: ctx.onerror,
      }),
  },
  {
    id: 'homeassistant',
    name: 'Home Assistant',
    description: 'Sensors and switches, read with a long-lived access token',
    keywords: 'hass entities sensors',
    category: 'home',
    icon: 'house',
    tags: ['local'],
    kind: 'service',
    state: ({ settings, status }) => {
      const ha = settings.homeAssistant;
      if (!ha) return NOT_SET_UP;
      if (ha.unreadable) return { kind: 'problem', detail: 'The connection cannot be read on this server. Connect again.' };
      return sourceState(status['homeassistant.default'], NOT_FETCHED);
    },
    panel: (ctx) =>
      view(HomeAssistantSetting, {
        connection: ctx.settings.homeAssistant,
        status: ctx.status['homeassistant.default'],
        onchanged: ctx.onchanged,
        onerror: ctx.onerror,
      }),
  },
  {
    id: 'homey',
    name: 'Homey',
    description: 'Device values and who is home, from a Homey Pro of any year',
    keywords: 'athom homeyscript presence',
    category: 'home',
    icon: 'people',
    tags: ['local', 'push'],
    kind: 'service',
    state: ({ settings, status }) => {
      const homey = settings.homey;
      if (!homey) return NOT_SET_UP;
      if (homey.unreadable) return { kind: 'problem', detail: 'The connection cannot be read on this server. Connect again.' };
      if (homey.mode === 'push') {
        const at = status['homey.snapshot']?.updatedAt;
        return { kind: isSnapshotLate(at) ? 'problem' : 'connected', detail: describeSnapshot(at) };
      }
      return sourceState(status['homey.default'] ?? status['homey.users'], NOT_FETCHED);
    },
    panel: (ctx) =>
      view(HomeySetting, {
        connection: ctx.settings.homey,
        status: ctx.settings.homey?.mode === 'push' ? ctx.status['homey.snapshot'] : (ctx.status['homey.default'] ?? ctx.status['homey.users']),
        onchanged: ctx.onchanged,
        onmanagetokens: () => ctx.onopen('apitokens'),
        onerror: ctx.onerror,
      }),
  },
  {
    id: 'photos',
    name: 'Photos',
    description: 'A slideshow of photos you upload; they stay on this server',
    keywords: 'pictures slideshow upload',
    category: 'media',
    icon: 'photo',
    tags: ['local'],
    kind: 'service',
    state: ({ photoCount }) => (photoCount > 0 ? { kind: 'connected', detail: `${count(photoCount, 'photo', 'photos')}.` } : NOT_SET_UP),
    panel: (ctx) => view(PhotoSetting, { onerror: ctx.onerror }),
  },
  {
    id: 'immich',
    name: 'Immich',
    description: 'Photos from an album on your own Immich server',
    keywords: 'photos album self-hosted',
    category: 'media',
    icon: 'photo',
    tags: ['local'],
    kind: 'service',
    requires: { setting: 'immich' },
    state: ({ settings, status }) => {
      const immich = settings.immich;
      if (!immich) return NOT_SET_UP;
      if (immich.unreadable) return { kind: 'problem', detail: 'The connection cannot be read on this server. Connect again.' };
      if (!immich.albumId) return { kind: 'problem', detail: 'Connected; choose an album.' };
      const st = sourceState(status['photos.immich'], 'Read once a Photos widget shows it.');
      return { ...st, detail: `${immich.albumName}. ${st.detail}` };
    },
    panel: (ctx) =>
      view(ImmichSetting, { connection: ctx.settings.immich, status: ctx.status['photos.immich'], onchanged: ctx.onchanged, onerror: ctx.onerror }),
  },
  {
    id: 'synologyphotos',
    name: 'Synology Photos',
    description: 'Photos from a shared album on your Synology, by its link',
    keywords: 'photos album nas dsm diskstation shared link',
    category: 'media',
    icon: 'photo',
    tags: ['local'],
    kind: 'service',
    requires: { setting: 'synologyPhotos' },
    state: ({ settings, status }) => {
      const album = settings.synologyPhotos;
      if (!album) return NOT_SET_UP;
      if (album.unreadable) return { kind: 'problem', detail: 'The album cannot be read on this server. Connect it again.' };
      const st = sourceState(status['photos.synology'], 'Read once a Photos widget shows it.');
      return { ...st, detail: `Shared album on ${album.host}. ${st.detail}` };
    },
    panel: (ctx) =>
      view(SynologySetting, {
        album: ctx.settings.synologyPhotos,
        status: ctx.status['photos.synology'],
        onchanged: ctx.onchanged,
        onerror: ctx.onerror,
      }),
  },
  {
    id: 'camera',
    name: 'Camera snapshots',
    description: 'A still from a camera at home, refreshed on the screen',
    keywords: 'snapshot doorbell cctv frigate reolink hikvision',
    category: 'home',
    icon: 'camera',
    tags: ['local'],
    kind: 'service',
    requires: { widget: 'camera' },
    // A camera is asked only while a screen shows it, so it has no
    // status of its own: Test under its setup asks it now.
    state: ({ cameraCount }) =>
      cameraCount > 0 ? { kind: 'connected', detail: `${count(cameraCount, 'camera', 'cameras')}. Asked while a screen shows one.` } : NOT_SET_UP,
    panel: (ctx) => view(CameraSetting, { onchanged: ctx.onchanged, onerror: ctx.onerror }),
  },
  {
    id: 'custom',
    name: 'Custom APIs',
    description: 'Any API that answers with JSON, or JSON your own system sends',
    keywords: 'json rest http webhook node-red values',
    category: 'own',
    icon: 'braces',
    tags: ['push'],
    kind: 'service',
    state: ({ sources, status }) => {
      if (sources.length === 0) return NOT_SET_UP;
      const unreadable = sources.find((s) => s.unreadable);
      if (unreadable) return { kind: 'problem', detail: `The secrets of ${unreadable.name} cannot be read on this server. Edit it.` };
      const failing = sources.find((s) => s.mode === 'fetch' && status[`custom.source.${s.id}`]?.failing);
      if (failing) return { kind: 'problem', detail: `${failing.name} could not be fetched. Test it under Custom APIs.` };
      const late = sources.find((s) => s.mode === 'push' && isPushLate(status[`custom.source.${s.id}`]?.updatedAt ?? s.lastPushAt, s.intervalSeconds));
      if (late) return { kind: 'problem', detail: `${late.name} sends no data; the screens mark it as old.` };
      return { kind: 'connected', detail: `${count(sources.length, 'API', 'APIs')}.` };
    },
    panel: (ctx) =>
      view(CustomSourceSetting, { status: ctx.status, onchanged: ctx.onchanged, onmanagetokens: () => ctx.onopen('apitokens'), onerror: ctx.onerror }),
  },
  {
    id: 'apitokens',
    name: 'API tokens',
    description: 'Let automations send data and announcements to GDash',
    keywords: 'token automation announcement bearer home assistant homeyscript push',
    category: 'own',
    icon: 'key',
    tags: ['push'],
    kind: 'service',
    state: ({ tokenCount }) => (tokenCount > 0 ? { kind: 'connected', detail: `${count(tokenCount, 'token', 'tokens')}.` } : NOT_SET_UP),
    panel: (ctx) => view(ApiTokenSetting, { onerror: ctx.onerror }),
  },
];

// Built-in sources: no setup, a widget is all it takes. Each shows only when
// the server reports its widget type, so the cards of the sources that are
// still being built (rain, warnings, holidays, moon) appear by
// themselves once they land.
const BUILT_IN: Integration[] = [
  {
    id: 'airquality',
    name: 'Air quality & pollen',
    description: 'Air quality and pollen at your place, from Open-Meteo',
    keywords: 'open-meteo pollen hay fever',
    category: 'weather',
    icon: 'air',
    tags: ['no-key'],
    kind: 'builtin',
    requires: { widget: 'airquality' },
    state: ({ settings, status }) => (settings.location ? builtinState(status['airquality.current']) : NEEDS_LOCATION),
  },
  {
    id: 'sun',
    name: 'Sunrise & sunset',
    description: 'Today’s sunrise and sunset at your place',
    keywords: 'open-meteo daylight',
    category: 'weather',
    icon: 'sun',
    tags: ['no-key'],
    kind: 'builtin',
    requires: { widget: 'sun' },
    state: ({ settings }) => (settings.location ? builtinState(undefined) : NEEDS_LOCATION),
  },
  {
    id: 'rain',
    name: 'Rain radar',
    description: 'Rain in the next two hours, from Buienradar',
    keywords: 'buienradar precipitation',
    category: 'weather',
    icon: 'rain',
    tags: ['no-key', 'nl'],
    kind: 'builtin',
    requires: { widget: 'rain' },
    state: ({ settings, status }) => (settings.location ? builtinState(status['rain.default']) : NEEDS_LOCATION),
  },
  {
    id: 'warnings',
    name: 'Weather warnings',
    description: 'Code yellow, orange and red for your province, from the KNMI',
    keywords: 'knmi code alert storm',
    category: 'weather',
    icon: 'alert',
    tags: ['no-key', 'nl'],
    kind: 'builtin',
    requires: { widget: 'warnings' },
    state: ({ status }) => builtinState(status['warnings.default']),
  },
  {
    id: 'moon',
    name: 'Moon',
    description: 'The phase of the moon tonight',
    keywords: 'lunar phase full moon',
    category: 'weather',
    icon: 'moon',
    tags: ['no-key'],
    kind: 'builtin',
    requires: { widget: 'moon' },
    state: () => builtinState(undefined),
  },
  {
    id: 'departures',
    name: 'Departures',
    description: 'The next buses, trams and trains from a stop, from OVapi',
    keywords: 'ovapi bus train tram public transport',
    category: 'travel',
    icon: 'bus',
    tags: ['no-key', 'nl'],
    kind: 'builtin',
    requires: { widget: 'departures' },
    state: ({ status }) => builtinState(status['departures.default']),
  },
  {
    id: 'fuel',
    name: 'Fuel prices',
    description: 'The cheapest petrol, diesel and LPG nearby, from the ANWB',
    keywords: 'anwb petrol diesel lpg euro95',
    category: 'travel',
    icon: 'fuel',
    tags: ['no-key', 'nl'],
    kind: 'builtin',
    requires: { widget: 'fuel' },
    state: ({ status }) => builtinState(status['fuel.default']),
  },
  {
    id: 'energyprice',
    name: 'Electricity prices',
    description: 'Today’s hourly electricity prices, from EnergyZero',
    keywords: 'energyzero dynamic tariff epex',
    category: 'energy',
    icon: 'bolt',
    tags: ['no-key', 'nl'],
    kind: 'builtin',
    requires: { widget: 'energyprice' },
    state: ({ status }) => builtinState(status['energyprice.nl']),
  },
  {
    id: 'waste',
    name: 'Waste collection',
    description: 'The next pickups, from your municipality’s waste calendar',
    keywords: 'afval gft bin ical pickup',
    category: 'lists',
    icon: 'bin',
    tags: ['no-key'],
    kind: 'builtin',
    requires: { widget: 'waste' },
    state: ({ settings, status }) => (settings.calendars.length > 0 ? builtinState(status['waste.default']) : NEEDS_CALENDAR),
  },
  {
    id: 'holidays',
    name: 'School & public holidays',
    description: 'The next school holiday for your region and public holidays',
    keywords: 'schoolvakantie feestdag vacation',
    category: 'lists',
    icon: 'calendar',
    tags: ['no-key', 'nl'],
    kind: 'builtin',
    requires: { widget: 'holidays' },
    state: ({ status }) => builtinState(status['holidays.nl']),
  },
  {
    id: 'nowplaying',
    name: 'Now playing (Cast)',
    description: 'What plays on your Cast speakers, groups and Nest Hubs, read only',
    keywords: 'chromecast google cast nest speaker spotify youtube music plex album cover',
    category: 'media',
    icon: 'music',
    tags: ['local', 'no-key'],
    kind: 'builtin',
    requires: { widget: 'nowplaying' },
    state: ({ status }) => builtinState(status['nowplaying.default']),
  },
];

function presetIntegration(preset: Preset): Integration {
  return {
    id: `preset-${preset.id}`,
    name: preset.name,
    description: preset.description,
    keywords: 'preset custom api',
    category: preset.category,
    icon: preset.icon,
    tags: preset.tags,
    kind: 'preset',
    requires: { widget: 'custom' },
    state: ({ sources, status }) => {
      const made = sourcesFrom(preset, sources)[0];
      if (!made) return { kind: 'idle', detail: 'Preset · Custom API' };
      const st = sourceState(status[`custom.source.${made.id}`], NOT_FETCHED);
      return { ...st, detail: `As ${made.name}. ${st.detail}` };
    },
    panel: (ctx) =>
      view(PresetSetup, { preset, made: sourcesFrom(preset, ctx.sources), onchanged: ctx.onchanged, onopen: ctx.onopen, onerror: ctx.onerror }),
  };
}

export const INTEGRATIONS: Integration[] = [...SERVICES, ...BUILT_IN, ...PRESETS.map(presetIntegration)];
