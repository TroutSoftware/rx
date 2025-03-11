import Go from "./wasm_exec.mjs";
import { OpType } from "./optype_abi";
import { IntentType } from "./intenttype_abi";
import { EventsCodes } from "../webc/datacell-abi";
import { listFlags } from "../utils/flags";

type registers = [r1: any, r2: any, r3: any, r4: any];
export type modifiers = [ctrl: boolean, shift: boolean, alt: boolean];
type FourArgs = [c1: any, c2: any, c3: any, c4: any];

type World = {
  registers: registers;
  mouse: [x: number, y: number];
  modifiers: modifiers;
  gen: number;
  continuation?: (args: FourArgs) => void;
};

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
class Renderer extends HTMLElement {
  module: Promise<WebAssembly.Module>;

  // All CSS style sheets of the parent are automatically inherited on the element.
  // This enables component-oriented frameworks surch as [tailwind] to work natively.
  // We use the new Constructable StyleSheets API of [CSSStyleSheet], which is well integrated in shadow DOM.
  //
  // [tailwind]: https://tailwindcss.com/
  // [CSSStyleSheet]: https://developer.mozilla.org/en-US/docs/Web/API/CSSStyleSheet
  static cachedStyleSheet: CSSStyleSheet;
  shadowRoot: ShadowRoot; // asserted in constructor

  go: Go;
  gen = 0;
  mxevent: boolean = false; // protects event loop task
  modifiers: modifiers = [false, false, false];
  mouse: [x: number, y: number];

  activeModule: Promise<void>;
  _tripModule: () => void;
  running = true;

  // this is installed during the module instanciation
  // see main_js.go
  updateGo: {
    (event: IntentType, entity: number, world: World): void;
    (event: IntentType.Seppuku): void;
  };

  constructor() {
    super();
    const shadow = this.attachShadow({ mode: "open" });

    // adopted CSS styles
    if (!Renderer.cachedStyleSheet) {
      console.time("caching CSS styles");
      Renderer.cachedStyleSheet = new CSSStyleSheet();
      for (const sheet of document.styleSheets) {
        for (const r of sheet.cssRules) {
          Renderer.cachedStyleSheet.insertRule(
            r.cssText,
            Renderer.cachedStyleSheet.cssRules.length,
          );
        }
      }
      console.timeEnd("caching CSS styles");
    }
    shadow.adoptedStyleSheets = [Renderer.cachedStyleSheet];

    // WASM module (use the `wasm-url` attribute on the element)
    console.time("compile module");
    const WASM_URL = this.getAttribute("wasm-url");
    if (!WASM_URL) {
      throw Error("Missing wasm-url attribute in node");
    }
    this.module = WebAssembly.compileStreaming(fetch(WASM_URL));
    console.timeEnd("compile module");
    this.activeModule = new Promise((resolve) => (this._tripModule = resolve));
  }

  async connectedCallback() {
    /**
     * Side-effect: will add WASM Scope-tied methods to "this"
     * @see main_js.go
     */

    this.shadowRoot.addEventListener("click", this);
    this.shadowRoot.addEventListener("dblclick", this);
    this.shadowRoot.addEventListener("contextmenu", this);
    this.shadowRoot.addEventListener("change", this);
    this.shadowRoot.addEventListener("focusout", this);
    // document-tied events, must be removed in disconnectedCallback
    document.addEventListener("mousemove", this);

    const env: { [key: string]: string } = {};
    env["FLAGS"] = Object.keys(await listFlags()).join(",");
    for (const key in this.dataset) {
      env[key] = this.dataset[key]!;
    }
    this.go = new Go([], env, this as any);
    await this.module
      .then((module) => WebAssembly.instantiate(module, this.go.importObject))
      .then((obj) => this.go.run(obj, this._tripModule));

    this.activeModule = new Promise((resolve) => {
      this._tripModule = resolve;
    });
  }

  disconnectedCallback() {
    this.running = false;

    (async () => {
      await this.activeModule;
      this.updateGo(IntentType.Seppuku);
    })();
    document.removeEventListener("mousemove", this);
  }

  // locateEntity provides an extension point to inject custom entity locator code.
  // The default option is to use the DOM tree, but other options are possible (e.g. based on geometric distance).
  locateEntity = ancestorOf;

  async handleEvent(event: Event) {
    if (isClick(event) || isRightClick(event) || isDoubleClick(event)) {
      assertsMouseEvent(event);
      event.preventDefault();

      // capture the mouse, in case the events gets delayed too much
      const mouse: [x: number, y: number] = [event.clientX, event.clientY];

      const target = this.locateEntity(event.target);
      if (target == null) {
        return;
      }

      // order matters: since the event detail can be used to differentiate,
      // a double click is a click with a count of 2.
      let code: IntentType;
      if (isDoubleClick(event)) {
        code = IntentType.DoubleClick;
      } else if (isClick(event) && isSubmitButton(event.target)) {
        if (!event.target.form) {
          throw Error("submit button without an associated form");
        }

        const data = new FormData(event.target.form);
        const queryString = new URLSearchParams(data as any).toString();
        this.passEvent(IntentType.Submit, this.locateEntity(event.target), {
          registers: [queryString || "", "", "", ""],
        });
        return;
      } else if (isClick(event)) {
        code = IntentType.Click;
      } else if (isRightClick(event)) {
        return;
      }

      const [act, name, buffer, _] = await new Promise<FourArgs>(
        (continuation) =>
          this.passEvent(code, this.locateEntity(target), {
            mouse,
            continuation,
          }),
      );
    } else if (event.type === "change") {
      /*if (!!(event.target as HTMLElement).closest('form')) {
        let data = new FormData((event.target as HTMLElement).closest('form'))
        // console.log("Data: ", data.get("description"))
        this.passEvent(IntentType.Change, this.locateEntity(event.target), {
          registers: [(event.target as HTMLInputElement).value || data.get("description"), (event.target as HTMLElement).nodeName, "", ""],
        });
      }*/
      this.passEvent(IntentType.Change, this.locateEntity(event.target), {
        registers: [(event.target as HTMLInputElement).value || "", "", "", ""],
      });
    } else if (event.type === "focusout") {
      if (!!(event.target as HTMLElement).closest("form")) {
        let data = new FormData((event.target as HTMLElement).closest("form"));
        this.passEvent(IntentType.Blur, this.locateEntity(event.target), {
          registers: [
            (event.target as HTMLInputElement).value || data.get("description"),
            (event.target as HTMLElement).nodeName,
            "",
            "",
          ],
        });
      } else {
        // remove this condition if needing to track 'blur' event
        const val = (event.target as any)?.value;
        this.passEvent(IntentType.Blur, this.locateEntity(event.target), {
          registers: [val || "", "", "", ""],
        });
      }
    } else if (isMouseMove(event)) {
      this.mouse = [event.clientX, event.clientY];
    } else if (isKeyDown(event)) {
      const entity = this.shadowRoot
        .elementFromPoint(this.mouse[0], this.mouse[1])
        ?.closest("[id]");
      if (!entity) {
        console.warn("no entity found, cannot attach event");
        return;
      }

      switch (event.code) {
        case "Escape":
          this.passEvent(EventsCodes.EscPress, entity);
          break;
        case "ArrowDown":
          event.preventDefault();
          this.passEvent(EventsCodes.Scroll, entity, {
            registers: ["1", "", "", ""],
          });
          break;
        case "ArrowUp":
          event.preventDefault();
          this.passEvent(EventsCodes.Scroll, entity, {
            registers: ["-1", "", "", ""],
          });
          break;
        case "F10":
          if (!event.ctrlKey) {
            console.debug("no meta key");
            return;
          }
          event.preventDefault();
          this.passEvent(
            EventsCodes.ShowDebugMenu,
            this.shadowRoot.querySelector("[id]")!,
          );
        default:
          return;
      }
    } else {
      console.log("Unknown event", event);
    }
  }

  async buildJSWorld(w: Partial<World>, e: Element): Promise<World> {
    return {
      mouse: this.mouse,
      modifiers: this.modifiers,
      gen: this.gen,
      registers: getRegisters(e),
      ...w,
    } as World;
  }

  passEvent = (
    eventType: IntentType,
    entityNode: Element | null,
    world: Partial<World> = {},
  ) => {
    if (!entityNode) {
      // many events are not captured from client code
      return;
    }

    if (
      !(entityNode instanceof HTMLElement || entityNode instanceof SVGElement)
    ) {
      throw new Error("event raised from entity which is not an HTML element");
    }

    // capture both entity and gen to prevent desync
    let entity = 0;
    if (entityNode.id === "") {
      throw new Error(
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
        const jsWorld = await this.buildJSWorld(evt.world, entityNode);
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

  redraw = (parr: Uint8Array) => {
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
            let p = this.shadowRoot.firstChild;
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

            const svgElementsMap = {
              // Existing map entries
              altglyph: "altGlyph",
              altglyphdef: "altGlyphDef",
              altglyphitem: "altGlyphItem",
              animatecolor: "animateColor",
              animatemotion: "animateMotion",
              animatetransform: "animateTransform",
              clippath: "clipPath",
              feblend: "feBlend",
              fecolormatrix: "feColorMatrix",
              fecomponenttransfer: "feComponentTransfer",
              fecomposite: "feComposite",
              feconvolvematrix: "feConvolveMatrix",
              fediffuselighting: "feDiffuseLighting",
              fedisplacementmap: "feDisplacementMap",
              fedistantlight: "feDistantLight",
              fedropshadow: "feDropShadow",
              feflood: "feFlood",
              fefunca: "feFuncA",
              fefuncb: "feFuncB",
              fefuncg: "feFuncG",
              fefuncr: "feFuncR",
              fegaussianblur: "feGaussianBlur",
              feimage: "feImage",
              femerge: "feMerge",
              femergenode: "feMergeNode",
              femorphology: "feMorphology",
              feoffset: "feOffset",
              fepointlight: "fePointLight",
              fespecularlighting: "feSpecularLighting",
              fespotlight: "feSpotLight",
              fetile: "feTile",
              feturbulence: "feTurbulence",
              foreignobject: "foreignObject",
              glyphref: "glyphRef",
              lineargradient: "linearGradient",
              radialgradient: "radialGradient",
              textpath: "textPath",

              // Common SVG elements
              svg: "svg",
              g: "g",
              defs: "defs",
              symbol: "symbol",
              use: "use",
              image: "image",
              path: "path",
              rect: "rect",
              circle: "circle",
              ellipse: "ellipse",
              line: "line",
              polyline: "polyline",
              polygon: "polygon",
              text: "text",
              tspan: "tspan",
              switch: "switch",
              marker: "marker",
              view: "view",
              mask: "mask",
              pattern: "pattern",
              filter: "filter",
              cursor: "cursor",
              metadata: "metadata",
              desc: "desc",
              title: "title",
              animate: "animate",
              set: "set",
              style: "style",
              script: "script",
            };

            if (svgElementsMap.hasOwnProperty(tag)) {
              n = document.createElementNS(
                "http://www.w3.org/2000/svg",
                svgElementsMap[tag],
              );
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

customElements.define("rx-bootstrap", Renderer);

function getRegisters(targetNode?): registers {
  const registers = Array(4).fill("") as registers;
  if (!targetNode) {
    return registers;
  }

  for (let ri = 1; ri <= 4; ri++) {
    const registryNode = targetNode.closest(`[data-r${ri}]`);
    if (registryNode) {
      registers[ri - 1] = registryNode.dataset[`r${ri}`];
    }
  }
  return registers;
}

function ancestorOf(targetNode: EventTarget | null): HTMLElement | null {
  if (
    !(targetNode instanceof HTMLElement || targetNode instanceof SVGElement)
  ) {
    console.error(targetNode);
    throw new Error("invalid target node");
  }
  // Emulate the native behavior of label/input bind
  let label = targetNode;
  if (!(targetNode instanceof HTMLLabelElement)) {
    label = targetNode?.closest("label");
  }
  if (label instanceof HTMLLabelElement) {
    if (label.control != null) {
      return label.control;
    }
  }

  const closest = targetNode?.closest("[id]");
  if (!closest) {
    return null;
  }
  return closest as HTMLElement;
}

// Typescript checks with inference
const isClick = (ev: Event) => ev.type === "click";
const isDoubleClick = (ev: Event) =>
  ev.type === "dblclick" || (isClick(ev) && (ev as MouseEvent).detail == 2);
const isRightClick = (ev: Event) => ev.type === "contextmenu";
const isMouseMove = (ev: Event): ev is MouseEvent => ev.type === "mousemove";
const isKeyDown = (ev: Event): ev is KeyboardEvent => ev.type === "keydown";
const isSubmitButton = (
  target: EventTarget | null,
): target is HTMLInputElement =>
  target instanceof HTMLInputElement && target.type === "submit";

function assertsMouseEvent(ev: Event): asserts ev is MouseEvent {
  if (!(ev instanceof MouseEvent)) {
    throw new Error("event is not MouseEvent");
  }
}
