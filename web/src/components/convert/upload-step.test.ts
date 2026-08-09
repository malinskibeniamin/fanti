import { expect, test } from 'vitest';

import {
  isUploadWithinLimit,
  MAX_UPLOAD_BYTES,
} from '@/components/convert/upload-step';

test('rejects files larger than the server request limit', () => {
  expect(isUploadWithinLimit({ size: MAX_UPLOAD_BYTES })).toBe(true);
  expect(isUploadWithinLimit({ size: MAX_UPLOAD_BYTES + 1 })).toBe(false);
});
