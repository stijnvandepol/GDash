/** Where a Photos widget takes its photos from; internal/display holds the same options. */
export type PhotoSource = 'uploaded' | 'immich' | 'synology';

// The names and IDs the server sends; anything else in a snapshot is
// ignored rather than put in a URL.
const PATTERNS: Record<PhotoSource, RegExp> = {
  uploaded: /^[0-9a-f]{32}\.(jpg|png|webp)$/,
  immich: /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/,
  synology: /^[0-9]{1,15}-[0-9A-Za-z_]{1,64}-(xl|m)$/,
};

const ROUTES: Record<PhotoSource, string> = {
  uploaded: '/api/photos/',
  immich: '/api/immich/assets/',
  synology: '/api/synology/photos/',
};

/** The source a widget's settings choose; uploaded photos unless another is set. */
export function photoSource(settings: Record<string, unknown> | undefined): PhotoSource {
  const source = settings?.source;
  return source === 'immich' || source === 'synology' ? source : 'uploaded';
}

/** The addresses of the photos in an entry's value, in its order, for the source. */
export function photoUrls(value: unknown, source: PhotoSource): string[] {
  if (!Array.isArray(value)) return [];
  return value.filter((v): v is string => typeof v === 'string' && PATTERNS[source].test(v)).map((v) => ROUTES[source] + v);
}
