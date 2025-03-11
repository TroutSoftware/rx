const CHUNK_SIZE = 400;

// openPipe is the Javascript end of the stream
// the stream is pull-triggered
globalThis.trout_sftw_openPipe = (
  readInto: (buf: Uint8Array, cb: (written: number) => void) => void,
) =>
  new ReadableStream({
    async pull(c: ReadableByteStreamController) {
      if (!c.byobRequest) {
        throw new Error("this stream should only work with byob");
      }
      const v = c.byobRequest.view;
      if (v == null) {
        throw Error("empty byob view");
      }
      const buf = new Uint8Array(v.buffer, v.byteOffset, v.byteLength);
      const sz = await new Promise<number>((resolve) => readInto(buf, resolve));
      if (sz >= 0) {
        c.byobRequest.respond(sz);
      } else {
        c.close();
      }
    },
    autoAllocateChunkSize: CHUNK_SIZE,
    // TODO(rdo) cancel
    type: "bytes",
  });

export const triggerDownload = async (
  name: string,
  content: ReadableStream,
) => {
  // need to realize stream apparently :shrug:
  const rdr = content.getReader();
  const pump = new ReadableStream({
    start(ctrl) {
      return pump();
      async function pump() {
        const { done, value } = await rdr.read();
        if (done) {
          ctrl.close();
          return;
        }

        ctrl.enqueue(value);
        return pump();
      }
    },
  });

  const res = new Response(pump);
  const blob = await res.blob();

  var a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = name || "";
  document.body.appendChild(a);
  a.click();
  a.remove();
};
