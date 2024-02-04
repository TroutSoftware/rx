import { openSync, readFileSync, readSync } from 'fs'
import { argv, env } from 'process'

// install random source before importing
const devrandom = openSync("/dev/random")
globalThis.crypto = {
  getRandomValues(ary) { readSync(devrandom, ary) },
}
const Go = await import('../webc/wasm_exec.mjs')

const wasmBuffer = readFileSync('rx_test.wasm')
const go = new Go.default(argv.slice(2), env)
go.exit = process.exit
process.on("exit", (code) => { // Node.js exits if no event handler is pending
  if (code === 0 && !go.exited) {
    // deadlock, make Go print error and stack traces
    go._pendingEvent = { id: 0 };
    go._resume();
  }
});

const obj = await WebAssembly.instantiate(wasmBuffer, go.importObject)
await go.run(obj.instance, () => { })