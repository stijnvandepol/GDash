import { crc32, deflateSync } from 'node:zlib';

/**
 * A PNG of width x height whose pixels colour gives, for stand-ins of
 * cameras and photo servers: a picture without a binary file in the
 * repository.
 */
export function png(width: number, height: number, colour: (x: number, y: number) => [number, number, number]): Buffer {
  const rows = Buffer.alloc((width * 3 + 1) * height);
  for (let y = 0; y < height; y++) {
    const start = y * (width * 3 + 1);
    rows[start] = 0; // no filter
    for (let x = 0; x < width; x++) rows.set(colour(x, y), start + 1 + x * 3);
  }
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header.set([8, 2, 0, 0, 0], 8); // 8 bits, RGB
  return Buffer.concat([
    Buffer.from('\x89PNG\r\n\x1a\n', 'latin1'),
    chunk('IHDR', header),
    chunk('IDAT', deflateSync(rows)),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

function chunk(type: string, data: Buffer): Buffer {
  const length = Buffer.alloc(4);
  length.writeUInt32BE(data.length);
  const body = Buffer.concat([Buffer.from(type, 'latin1'), data]);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(body));
  return Buffer.concat([length, body, crc]);
}

/** A garden at dusk: sky above, a lawn below and a path, so a screenshot shows a picture. */
export const garden = (width = 320, height = 180) =>
  png(width, height, (x, y) => {
    const horizon = height * 0.55;
    if (y < horizon) return [40 + (y / horizon) * 120, 70 + (y / horizon) * 90, 140 - (y / horizon) * 30];
    const onPath = Math.abs(x - width / 2) < (y - horizon) * 0.6;
    return onPath ? [150, 130, 100] : [50, 110 - ((y - horizon) / height) * 60, 45];
  });
