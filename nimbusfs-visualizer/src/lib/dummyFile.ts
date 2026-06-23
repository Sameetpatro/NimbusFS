import { formatBytes } from './chunking';

/** 10 MB — produces 3 chunks at 4 MiB each (matches README example). */
const DUMMY_SIZE = 10 * 1024 * 1024;
const DUMMY_NAME = 'vacation-photos.zip';

let cachedDummy: File | null = null;

export interface DummyMeta {
  name: string;
  size: number;
  sizeLabel: string;
  chunks: number;
}

export const DUMMY_FILE_META: DummyMeta = {
  name: DUMMY_NAME,
  size: DUMMY_SIZE,
  sizeLabel: formatBytes(DUMMY_SIZE),
  chunks: 3,
};

export function createDummyFile(): File {
  if (cachedDummy) return cachedDummy;

  const buffer = new Uint8Array(DUMMY_SIZE);
  for (let i = 0; i < DUMMY_SIZE; i++) {
    buffer[i] = (i * 7 + (i >> 8)) & 0xff;
  }

  cachedDummy = new File([buffer], DUMMY_NAME, { type: 'application/zip' });
  return cachedDummy;
}
