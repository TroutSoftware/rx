import { OpType } from "./optype_abi";
import { IntentType } from "./intenttype_abi";

type registers = [r1: string, r2: string, r3: string, r4: string];
type modifiers = [ctrl: boolean, shift: boolean, alt: boolean];

type World = {
    registers: registers;
    mouse: [x: number, y: number];
    modifiers: modifiers;
    point?: number;
    gen: number;
    continuation?: (args: FourArgs) => void;
};

type FourArgs = [r1: any, r2: any, r3: any, r4: any];

/**
 * Renderer is the wrapper around the Go WASM code.
 * It works as a shim to collect events and viewport information,
 * then pass to the engine for rendering.
 *
 * # Concurrency model
 *
 * The view is protected by an optimistic locking scheme:
 * an event is only processed if from the current gen.
 *
 * This guaranties that the tree structure in the view is the same than the one which raises the event.
 * Other properties (mouse position, viewport, scroll, …) are not tracked, and should be captured in the handler if desired.
 *
 * Note that changes to the gen can happen from Go, without JS calls:
 * this is the case when the network triggers the change.
 * 
 * # Implementing subclasses
 * 
 * The Renderer class is responsible for the low-level synchronisation with the underlying Go engine
 * (render the DOM upon request, and pass events back to the engine for a state update).
 * 
 * Concrete implementations of the Renderer must implement the event handler to react to events,
 * eventually calling the `passEvent` method to call into the Go / WASM engine
 * (note how the promise syntax can be used to capture values returned from Go).
 *  
 * ```
 *  const [act, name, buffer, _] = await new Promise<FourArgs>(
 *      (continuation) =>
 *          this.passEvent(code, parentEntity(target), { point, continuation }),
 *  );
 *  if (act === "trigger-download") {
 *      triggerDownload(name, buffer);
 *  } else {
 *      // do nothing
 *  }
 * ```
 * 
 * Similarly, `buildJSWorld` should be implemented to capture different elements of the UI to feed to Go.
 * A simple example mixin class would track the position of the mouse, and set it when required:
 * 
 * ```
 * class MouseTracker {
 *   mouse: [x: number, y: number];
 * 
 *   setMousePosition(w: Partial<World>): Partial<World> {
 *      return {
 *        mouse: this.mouse,
 *        ...w
 *      }
 *   }
 * }
 * ```
 */
export default abstract class Renderer extends HTMLElement {
    static module: Promise<WebAssembly.Module>;

    endStyle: HTMLElement;
    shadowRoot: ShadowRoot; // asserted in constructor
    
    gen = 0;
    debounce: boolean = false; // protects debounce process
    mxevent: boolean = false; // protects event loop task

    activeModule: Promise<void>;
    _tripModule: () => void;
    running = true;
    abstract WASM_URL(): string; // URL of the WASM module

    // this is installed during the module instanciation
    // see main_js.go
    updateGo: {
        (event: IntentType, entity: number, world: World): void;
        (event: IntentType.Seppuku): void;
    };
    
    constructor() {
        super();
        const shadow = this.attachShadow({ mode: "open" });
        for (const sheet of document.styleSheets) {
            if (!sheet.href) {
                continue;
            }

            let link = document.createElement("link");
            link.href = sheet.href;
            link.rel = "stylesheet";

            shadow.appendChild(link);
        }

        this.endStyle = shadow.lastChild as HTMLElement;
        if (!Renderer.module) {
            console.time("compile module");
            Renderer.module = WebAssembly.compileStreaming(fetch(this.WASM_URL()));
            console.timeEnd("compile module");
        }
        this.activeModule = new Promise((resolve) => (this._tripModule = resolve));
    }

    abstract handleEvent(event: Event): void;
    abstract buildJSWorld(w: Partial<World>): Promise<World>;

    passEvent = (
        eventType: IntentType,
        entityNode: Element,
        world: Partial<World> = {},
    ) => {
        if (
            !(entityNode instanceof HTMLElement || entityNode instanceof SVGElement)
        ) {
            throw new Error("event raised from entity which is not an HTML element");
        }

        // capture both entity and gen to prevent desync
        let entity = 0;
        if (entityNode.id === "") {
            console.warn(
                `entityId received an entityNode without id attribute: ${entityNode}`,
            );
            return;
        } else {
            entity = parseInt(entityNode.id);
            if (isNaN(entity)) {
                throw new Error(
                    `node is using a non-number id "${entityNode.id}". Use "GiveKey()" in Go code to generate a valid id for an input`,
                );
            }
        }

        const gen = this.gen;
        if (!this.shadowRoot.getElementById(entity.toString())) {
            // event was fired from a node has been deleted since
            // this is possible if the event is fired between the call to updateGo and the time the new rendering is done
            return;
        }

        if (this.mxevent) {
            // another event is being processed
            return;
        }

        this.mxevent = true;
        const sched = async () => {
            try {
                const evt = {
                    gen: gen,
                    code: eventType,
                    entity: entity,
                    world,
                };
                const jsWorld = await this.buildJSWorld(evt.world);
                await this.activeModule; // Go is ready to accept args

                // all must be sync below this point
                if (evt.gen === this.gen) {
                    this.updateGo(eventType, entity, jsWorld);
                } else {
                    // drop this event
                }
            } finally {
                this.mxevent = false;
            }
        };

        // note that the call, while using the async syntax, can actually be synchronous if the LioLi computation is synchronous.
        // this is the case if the [viewportToLioLi] is not using the visual debugger.
        // keeping this synchronous is required when we handle drag start event, where the payload and image must be found in the same loop.
        //
        // [dnd] https://html.spec.whatwg.org/multipage/dnd.html#concept-dnd-rw
        sched();
    };


    buildView = (parr: Uint8Array) => {
        // Use fat arrow syntax to make sure "this" is bound to instance
        const program = new DataView(parr.buffer);

        // offsets
        const instr_size = 1;
        const str_size = 2;

        const decoder = new TextDecoder("utf-8");

        let ip = 0;
        const ndoc = new DocumentFragment();
        let anchor: DocumentFragment | Element | any = ndoc; // covers initialization weirdness
        let next = anchor.firstChild;
        const loadString = () => {
            const len = program.getUint16(ip);
            ip += str_size;
            const txt = decoder.decode(new DataView(program.buffer, ip, len));
            ip += len;
            return txt;
        };

        while (ip < program.byteLength) {
            const instr = program.getUint8(ip);
            ip += instr_size;
            switch (instr) {
                case OpType.OpTerm:
                    for (
                        let p = this.endStyle.nextSibling;
                        p !== null;
                        p = p.nextSibling
                    ) {
                        p.remove();
                    }
                    this.shadowRoot.appendChild(ndoc);
                    this.gen++;
                    return;
                case OpType.OpCreateElement:
                    {
                        const tag = loadString();
                        let n: Element;
                        if (tag === "svg" || tag === "path") {
                            n = document.createElementNS("http://www.w3.org/2000/svg", tag);
                        } else {
                            n = document.createElement(tag);
                        }
                        if (next) {
                            next.replaceWith(n);
                        } else {
                            anchor.appendChild(n);
                        }
                        next = n.firstChild;
                        anchor = n;
                    }
                    break;
                case OpType.OpReuse:
                    {
                        const ntt = loadString();
                        const n = this.shadowRoot.getElementById(ntt);
                        if (next) {
                            next.replaceWith(n);
                        } else if (n) {
                            anchor.appendChild(n);
                        } else {
                            throw new Error(`Couldn't reuse node of id '${ntt}', not found`);
                        }
                        next = n!.nextSibling;
                    }
                    break;
                case OpType.OpReID:
                    {
                        const from = loadString();
                        const to = loadString();
                        const n = ndoc.getElementById(from);
                        n!.id = to;
                    }
                    break;
                case OpType.OpSetClass:
                    {
                        const cname = loadString();
                        anchor.setAttribute("class", cname);
                    }
                    break;
                case OpType.OpSetID:
                    {
                        const ntt = loadString();
                        anchor.id = ntt;
                    }
                    break;
                case OpType.OpSetAttr:
                    {
                        const anm = loadString();
                        const avl = loadString();
                        // setAttribute lets us use the normal HTML name of the attribute
                        anchor.setAttribute(anm, avl);
                    }
                    break;
                case OpType.OpAddText:
                    {
                        const txt = loadString();
                        if (next) {
                            next.replaceWith(txt);
                        } else {
                            anchor.appendChild(document.createTextNode(txt));
                        }
                    }
                    break;
                case OpType.OpNext:
                    {
                        next = anchor.nextSibling;
                        anchor = anchor.parentElement;
                    }
                    break;
            }
        }

        throw new Error("invalid XAS code, no term instructions");
    };

}