/** Saves bytes to the user's device via a temporary object URL. */
export function saveFile(filename: string, data: Uint8Array) {
  const blob = new Blob([data.slice().buffer], {
    type: 'application/octet-stream',
  });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}
