// Shapes of the data the GDash API sends and receives. api.ts re-exports
// them, so components import everything from './api'.
import type { ScreenReport } from '../screenReport';

export type { ScreenReport };
export interface User {
  id: number;
  username: string;
}

export interface Display {
  id: string;
  name: string;
  castDeviceId: string;
  createdAt: string;
  /**
   * Screens paired by code; only present in the display list. The TV that
   * GDash casts to is not one of them.
   */
  pairedScreens?: number;
  /** Empty means the default dashboard. */
  dashboardId: string;
  /** Active hours as HH:MM; both empty means always on. */
  activeFrom: string;
  activeUntil: string;
  /** Other dashboards during parts of the day; the first matching slot wins. */
  schedule: Slot[];
  /** Dashboards shown in turn with the display's own, each for rotateSeconds. */
  rotation: string[];
  rotateSeconds: number;
  /** Text size in percent: 100, 125, 150 or 200. */
  textScale: number;
  /** How often the receiver on a Cast device is restarted against the TV's screensaver, in minutes; 0 is never. */
  screensaverMinutes: number;
  /** What the screen does for its panel: "off", or "oled" against burn-in. */
  screenCare: ScreenCare;
  /** When the screen is dimmed to 60 %: "off", "sunset" (until sunrise) or "hours" (dimFrom to dimUntil, HH:MM). */
  dimAtNight: DimAtNight;
  dimFrom: string;
  dimUntil: string;
  /** What GDash last found on the Cast device; only in the display list. */
  cast?: CastStatus;
  /** What the display's screens last reported about their browsers, newest first; only in the display list. */
  screenReports?: ReceivedScreenReport[];
}

export type ScreenCare = 'off' | 'oled';
export type DimAtNight = 'off' | 'sunset' | 'hours';
/** A change to how a display looks after its screen. */
export type ScreenCareChanges = Partial<Pick<Display, 'screenCare' | 'dimAtNight' | 'dimFrom' | 'dimUntil'>>;

/** A screen's report about its browser as the display list carries it. */
export interface ReceivedScreenReport extends ScreenReport {
  /** "cast" for the TV GDash casts to, "screen" for one paired with a code. */
  screen: 'cast' | 'screen';
  receivedAt: string;
}

export interface CastStatus {
  state: 'connecting' | 'unreachable' | 'showing' | 'standby' | 'other_app' | 'yielded' | 'resting';
  since: string;
  /** Why the device cannot be reached, in the words of the error. */
  problem?: string;
  /** What kind of problem: not found on the network, or found but not answering. */
  reason?: 'not_found' | 'no_answer';
  /** When GDash last restarted the receiver against the screensaver, and why. */
  restartedAt?: string;
  restartReason?: 'periodic';
  /** When the page on the TV last said it was hidden, such as by an app or a screensaver. */
  hiddenAt?: string;
}

/** A dashboard shown from one time of day until another, as HH:MM. */
export interface Slot {
  from: string;
  until: string;
  /** mon to sun; empty means every day. */
  days: string[];
  dashboardId: string;
}

export interface Widget {
  id: string;
  type: string;
  x: number;
  y: number;
  w: number;
  h: number;
  settings?: Record<string, unknown>;
}

export interface Dashboard {
  id: string;
  name: string;
  theme?: string;
  /** A typeface instead of the theme's own; empty for the theme's. */
  font?: string;
  /** The theme to switch to between sunset and sunrise at the weather location; empty keeps the theme. */
  nightTheme?: string;
  widgets: Widget[];
  /** When it was made; only in the dashboard list. */
  createdAt?: string;
}

/** A ready-made layout to start a dashboard from. */
export interface DashboardTemplate {
  id: string;
  name: string;
  description: string;
  theme: string;
  widgets: Widget[];
}

export interface WidgetCatalogue {
  columns: number;
  rows: number;
  themes: string[];
  fonts: string[];
  types: Record<string, WidgetType>;
}

export interface WidgetType {
  minW: number;
  minH: number;
  settings: Record<string, SettingRule>;
}

/** A setting a widget type takes; internal/display validates the same. */
export interface SettingRule {
  kind:
    | 'integer'
    | 'text'
    | 'date'
    | 'calendar'
    | 'camera'
    | 'entities'
    | 'homeyCapabilities'
    | 'homeyUsers'
    | 'castDevices'
    | 'newsFeeds'
    | 'stop'
    | 'place'
    | 'customSource'
    | 'webPage'
    | 'choice'
    | 'boolean'
    | 'choices'
    | 'p2000Places'
    | 'roads'
    | 'path'
    | 'paths'
    | 'pairs'
    | 'rules';
  /** Bounds of an integer, of a text's length in characters, or of how many entities or items. */
  min?: number;
  max?: number;
  required: boolean;
  /** The values a choice, or each of a list of choices, takes. */
  options?: string[];
}

export interface Location {
  name: string;
  latitude: number;
  longitude: number;
}

export interface NewsFeed {
  id: number;
  name: string;
  url: string;
}

/** A news feed GDash offers to choose from; internal/news/catalogue.json holds them. */
export interface NewsCatalogueEntry {
  id: string;
  name: string;
  /** ISO 639-1, such as nl. */
  language: string;
  /** ISO 3166-1 alpha-2, such as NL. */
  country: string;
  category: string;
  url: string;
  /** What is known of the publisher's terms, if anything. */
  note?: string;
}

export interface NewsCatalogue {
  categories: string[];
  sources: NewsCatalogueEntry[];
}

/** A calendar; its secret address never leaves the server. */
export interface Calendar {
  id: number;
  name: string;
  host: string;
  /** One of the names in colors.ts. */
  color: string;
  /** True when the address cannot be decrypted with this server's key. */
  unreadable: boolean;
}

/** One event a calendar test found, as the TV receives it. */
export interface CalendarEvent {
  title: string;
  /** An RFC 3339 time, or a date for an all-day event. */
  start: string;
  end: string;
  allDay: boolean;
}

/** What a test of a calendar address found in the next 30 days. */
export interface CalendarTest {
  /** The calendar's own name, or empty. */
  name: string;
  count: number;
  /** The first few events. */
  events: CalendarEvent[];
}

/** A place alarmeringen.nl knows, as the P2000 place test found it. */
export interface P2000Place {
  name: string;
  slug: string;
  /** The slug of its safety region, and that region's name. */
  region: string;
  regionName: string;
}

export interface Settings {
  location: Location | null;
  newsFeeds: NewsFeed[];
  /** The calendars of appointments, for the Calendar widget. */
  calendars: Calendar[];
  /** The waste collection calendars, for the Waste collection widget. */
  wasteCalendars: Calendar[];
  /** What the TVs show their texts and dates in. */
  language: 'en' | 'nl' | 'de' | 'fr';
  /** Null until Home Assistant is connected; the token never leaves the server. */
  homeAssistant: HomeAssistant | null;
  /** Null until Homey is connected; the API key never leaves the server. */
  homey: Homey | null;
  /** Null until Immich is connected; the API key never leaves the server. */
  immich: Immich | null;
  /** Null until a Synology Photos album is connected; its link and password never leave the server. */
  synologyPhotos: Synology | null;
  /** Whether GDash asks GitHub once a day for the latest release. */
  updateCheck: boolean;
  /** Null until the solar panels are set up. */
  solar: Solar | null;
  /** The switch for the unofficial ANWB traffic info. */
  traffic: Pick<TrafficInfo, 'enabled' | 'acknowledgedAt'>;
  /** Whether a KNMI key is set; the key never leaves the server. */
  knmi: Pick<KnmiInfo, 'connected' | 'unreadable'>;
}

/** A group of panels that face the same way (internal/solar). */
export interface SolarArray {
  /** Degrees from 0 (flat) to 90 (upright). */
  tilt: number;
  /** Degrees from -180 to 180: 0 south, -90 east, 90 west. */
  azimuth: number;
  kwp: number;
}

/** The household's solar panels, for the solar forecast widget. */
export interface Solar {
  /** Where the panels are, or null for the weather location. */
  location: Location | null;
  arrays: SolarArray[];
  /** 0.5 to 1.0; only the Open-Meteo estimate uses it. */
  performanceRatio: number;
  source: 'open-meteo' | 'forecast.solar';
}

export interface Immich {
  url: string;
  /** Empty until an album is chosen. */
  albumId: string;
  albumName: string;
  /** True when the connection cannot be decrypted with this server's key. */
  unreadable: boolean;
}

/** A Synology Photos album shared by link, as the web interface sees it. */
export interface Synology {
  /** The NAS's address and port. */
  host: string;
  passwordSet: boolean;
  /** The SHA-256 fingerprint of the accepted certificate as hex pairs, or empty. */
  certificate: string;
  /** True when the album cannot be decrypted with this server's key. */
  unreadable: boolean;
}

/** An Immich album to choose for the Photos widget. */
export interface ImmichAlbum {
  id: string;
  name: string;
  assetCount: number;
}

/** A camera on the home network; its secret never leaves the server. */
export interface Camera {
  id: number;
  name: string;
  /** The snapshot address, shown to admins only. */
  url: string;
  auth: { kind: CustomAuthKind; name: string; secretSet: boolean };
}

/** A camera as the form sends it; an empty secret keeps the stored one. */
export interface CameraInput {
  name: string;
  url: string;
  auth: { kind: CustomAuthKind; name: string; secret: string };
}

/** The result of a camera's Test button. */
export interface CameraTest {
  contentType: string;
  bytes: number;
}

export interface Homey {
  /** local: GDash asks the Homey with an API key; push: the Homey sends its data with HomeyScript. */
  mode: 'local' | 'push';
  /** Empty when the Homey sends its data. */
  url: string;
  /** True when the connection cannot be decrypted with this server's key. */
  unreadable: boolean;
}

/** A capability of a Homey device, as the widget editor lists it. */
export interface HomeyCapability {
  /** "<device id>/<capability id>", what a Homey widget stores. */
  key: string;
  zone?: string;
  device: string;
  title: string;
  capability: string;
  value: number | boolean | string | null;
  units?: string;
  decimals?: number;
}

/** A Homey user, as the Who's home editor lists them. */
export interface HomeyUser {
  id: string;
  name: string;
}

/** The running server, for admins. */
export interface SystemInfo {
  version: string;
  /** True when GDASH_UPDATE_CHECK=off turns the update check off, whatever the setting says. */
  updateCheckLocked: boolean;
  /** A newer release, when the daily check found one. */
  update?: Release;
}

export interface Release {
  /** Without a leading v, such as 0.8.0. */
  version: string;
  /** The release notes. */
  url: string;
}

export interface HomeAssistant {
  url: string;
  /** True when the connection cannot be decrypted with this server's key. */
  unreadable: boolean;
}

/** A Home Assistant entity, as the widget editor lists it. */
export interface HomeAssistantEntity {
  id: string;
  name: string;
  state: string;
  unit?: string;
}

/** A public transport stop area, as the stop search lists it. */
export interface DepartureStop {
  code: string;
  /** Includes the town, such as "Utrecht, CS Jaarbeurszijde". */
  name: string;
}

export type CustomAuthKind = 'none' | 'bearer' | 'header' | 'query' | 'basic' | 'login';

/** How a custom API that wants a session is logged in to; the password is the auth secret. */
export interface CustomLogin {
  /** On the same host as the API's address. */
  url: string;
  /** JSON with {secret} where the password goes. */
  body: string;
  /** Where the token is in the login's answer, such as session.sid. */
  tokenPath: string;
  /** In a header or address parameter named by the auth name, or as a bearer token. */
  place: 'header' | 'query' | 'bearer';
  /** Where the answer says how many seconds the token lasts, or empty. */
  validityPath: string;
}

/** An extra request header of a custom API; a secret one comes back without its value. */
export interface CustomHeader {
  name: string;
  value: string;
  secret: boolean;
}

/** A JSON API set up under Integrations; its secrets never leave the server. */
/** How GDash gets a custom API's answer: it fetches it, or the household's system sends it. */
export type CustomMode = 'fetch' | 'push';

export interface CustomSource {
  id: number;
  name: string;
  mode: CustomMode;
  /** Empty for a source that is sent to GDash. */
  url: string;
  method: 'GET' | 'POST';
  body: string;
  auth: { kind: CustomAuthKind; name: string; secretSet: boolean; login?: CustomLogin };
  headers: CustomHeader[];
  intervalSeconds: number;
  /** A pasted answer to design widgets against, or null. */
  sample: unknown;
  /** The values named under Integrations, which the dashboard editor offers by name. */
  values: NamedValue[];
  lists: NamedList[];
  /** The id of the preset it was set up with, such as homewizard-p1, or empty for the household's own API. */
  preset: string;
  /** True when the secrets cannot be decrypted with this server's key. */
  unreadable: boolean;
  /** When a source that is sent to GDash last sent its data; absent before the first time. */
  lastPushAt?: string;
}

/** A value of a custom API's answer with a name, such as "Outside" at current.temperature_2m. */
export interface NamedValue {
  name: string;
  path: string;
  unit?: string;
  /** Decimals a number shows; absent shows it as sent. */
  decimals?: number;
}

/** A list in a custom API's answer with up to three columns, each read from an item. */
export interface NamedList {
  name: string;
  path: string;
  columns: { name: string; path: string }[];
}

export interface CustomValues {
  values: NamedValue[];
  lists: NamedList[];
}

/** A custom API as the form sends it; an empty secret keeps the stored one. */
export interface CustomSourceInput {
  name: string;
  mode: CustomMode;
  url: string;
  method: 'GET' | 'POST';
  body: string;
  auth: { kind: CustomAuthKind; name: string; secret: string; login?: CustomLogin };
  headers: CustomHeader[];
  intervalSeconds: number;
  sample: unknown;
  /** The preset it is set up with; absent or empty for the household's own API. */
  preset?: string;
}

/** One answer of a custom API, cut down for the editor. */
export interface CustomTest {
  data: unknown;
  truncated: boolean;
  bytes: number;
  /** For a source that is sent to GDash: when this, its last data, arrived. */
  receivedAt?: string;
}

/** Whether a data source works, by data key such as "weather.current". */
export interface SourceStatus {
  updatedAt?: string;
  failing: boolean;
}

/** GET /api/traffic: the switch for the unofficial ANWB traffic info and how it goes. */
export interface TrafficInfo {
  enabled: boolean;
  /** When the admin last acknowledged the terms; absent if never. */
  acknowledgedAt?: string;
  anwb: {
    state: 'idle' | 'ok' | 'failing' | 'stopped';
    /** The status code of the last refusal. */
    httpStatus?: number;
    nextAttemptAt?: string;
  };
  /** Present once a Traffic widget is on a dashboard. */
  data?: SourceStatus;
  /** Present once a Traffic widget shows fixed cameras. */
  cameras?: SourceStatus;
}

/** GET /api/knmi: whether a key is set; the key never leaves the server. */
export interface KnmiInfo {
  connected: boolean;
  unreadable: boolean;
  /** Present once a Weather widget shows measured values. */
  data?: SourceStatus;
}

export interface Place extends Location {
  /** Province or state and country, for telling places apart. */
  region: string;
}

export interface Photo {
  name: string;
  size: number;
  addedAt: string;
}

/** An API token for automations; the secret is only in the response that makes it. */
export interface ApiToken {
  id: number;
  name: string;
  createdAt: string;
  /** Null until the token is used. */
  lastUsedAt: string | null;
}

export interface NewApiToken extends ApiToken {
  token: string;
}

/** The answer to a right password when two-step login is on: no session yet. */
export interface PendingLogin {
  pending: string;
}

export interface TwoStepStatus {
  enabled: boolean;
  recoveryCodesLeft: number;
}

/** A new authenticator secret; the server never returns it again. */
export interface TwoStepEnrolment {
  /** Base32, to type into the app. */
  secret: string;
  /** The otpauth:// address the QR code holds. */
  uri: string;
  /** The QR code as an SVG document, black on white. */
  qr: string;
}

export interface CastDevice {
  id: string;
  name: string;
  model: string;
  addr: string;
  hasScreen: boolean;
}

/** A web page set up under Settings, which a Web page widget shows in a frame. */
export interface WebPage {
  id: number;
  name: string;
  url: string;
  /** Minutes between reloads; 0 never reloads. */
  refreshMinutes: number;
  /** Size in percent, 50 to 200. */
  zoom: number;
  allowScripts: boolean;
}

export type WebPageInput = Pick<WebPage, 'name' | 'url' | 'refreshMinutes' | 'zoom' | 'allowScripts'>;

/** What a page's headers say about being shown in a frame; see internal/webpage. */
export interface WebPageCheck {
  status: number;
  /** The header that forbids GDash to frame the page; absent when it may. */
  blocked?: 'x-frame-options' | 'frame-ancestors';
  header?: string;
}
