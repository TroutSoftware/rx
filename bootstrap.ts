import Go from "./wasm_exec.mjs";
import { OpType } from "./optype_abi";
import { IntentType } from "./intenttype_abi";

type registers = [r1: any, r2: any, r3: any, r4: any];
export type modifiers = [ctrl: boolean, shift: boolean, alt: boolean];
type FourArgs = [c1: any, c2: any, c3: any, c4: any];
const DEBOUNCE_TIMEOUT = 60; // ms time range. Tuned to ~1 event / rendering cycle at 16 fps
const DRAG_FORMAT = "x-t.sftw/drag-data";
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
  debounce: boolean = false;
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
    // this.shadowRoot.addEventListener("change", this);
    this.shadowRoot.addEventListener("focusout", this);
    this.shadowRoot.addEventListener("dragover", this);
    this.shadowRoot.addEventListener("dragenter", this);
    this.shadowRoot.addEventListener("drop", this);
    this.shadowRoot.addEventListener("dragend", this);
    this.shadowRoot.addEventListener("dragstart", this);
    this.shadowRoot.addEventListener("keyup", this);
    // document-tied events, must be removed in disconnectedCallback
    document.addEventListener("mousemove", this);
    document.addEventListener("wheel", this, { passive: false });

    const env: { [key: string]: string } = {};
    for (const key in this.dataset) {
      env[key] = this.dataset[key]!;
    }

    let args = [];
    if (this.hasAttribute("page")) {
      args = ["-page", this.getAttribute("page"), ...args];
    }

    if (this.hasAttribute("initial-datum")) {
      args = ["-datum", this.getAttribute("initial-datum"), ...args];
    }

    this.go = new Go(args, env, this as any);
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

      let target = this.locateEntity(event.target)
      if (target == null) {
        return;
      }
      //TODO: resolve all events instead of just click
      const linked = target.dataset.linkedEntity;
      if (linked) {
          const newTarget = this.shadowRoot.getElementById(linked);
          if (newTarget) {
              newTarget.dispatchEvent(new MouseEvent("click", { bubbles: true }));
              return; //ignoring the handling in the origin element
          }
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

      // See /RedirectTo/
      if (act === "redirect") {
        window.open(name, "_self");
      }

      // See /CopyToClipboard/
      if (act === "copyToClipboard") {
        navigator.clipboard.writeText(name)
      }
    } else if (event.type === "change") {
      // change event only for input, select and textarea
      // https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/change_event
      // See /ReadInput/
      this.passEvent(IntentType.Change, this.locateEntity(event.target), {
        registers: [(event.target as HTMLInputElement).value || "", "", "", ""],
      });
    } else if (event.type === "focusout") {
        // focusout bubbles, so it is usually a safer alternative to blur
        // https://developer.mozilla.org/en-US/docs/Web/API/Element/focusout_event
        // See /ReadInput/
        this.passEvent(IntentType.Blur, this.locateEntity(event.target), {
          registers: [(event.target as any)?.value || "", "", "", ""],
        });
    } else if (isDragOver(event)) {
      if (!acceptDrop(event, (e) => event.dataTransfer!.types.includes(DRAG_FORMAT))) {
        return;
      }
      // we need to set those because of the prevent default, and mouse move is not fired during a drag
      this.mouse = [event.clientX, event.clientY];

      if (this.debounce) {
        return;
      } else {
        this.debounce = true;
        setTimeout(() => {
          this.debounce = false;
        }, DEBOUNCE_TIMEOUT);
      }

      const entity = (event.target as HTMLElement | SVGElement).closest("[id]");
      // silence drops over empty zones (but should we not fold this into accepting the drop in the first place?)
      if (!entity) return;

      // NOTE: this may rerender the dropzone, thus breaking drop events
      // a drop event is only fired if valid dragenter/dragover have been seen before
      // and the DOM element for the dropzone is not rerendered in between
      this.passEvent(IntentType.DragOver, entity);
    } else if (isDragEnter(event)) {
      acceptDrop(event, (e) => event.dataTransfer!.types.includes(DRAG_FORMAT));
    } else if (isDrop(event)) {
      if (!acceptDrop(event, (e) => event.dataTransfer!.types.includes(DRAG_FORMAT))) {
        return;
      }

      const data = event.dataTransfer!.getData(DRAG_FORMAT);
      if (!data) {
        console.warn("Did not get data, ignore drop");
        return;
      }

      const target = ancestorOf(event.target); //TODO try to use coordinate system
      if (target == null) {
        return;
      }
      this.passEvent(IntentType.Drop, target, {
        registers: [data, "", "", ""],
      });
    } else if (isDragEnd(event)) {
        const elem = event.target as HTMLElement | null;
      if (!elem) {
        return;
      }

      this.passEvent(IntentType.DragEnd, ancestorOf(event.target));
    } else if (isDragStart(event)) {
      const elem = event.target as HTMLElement | null;
      if (!elem) {
        return;
      }

      const entity = elem.closest("[id]")!;

      const [data,effect,image, _t] = await new Promise<FourArgs>((continuation) =>
          this.passEvent(IntentType.DragStart, entity, { continuation }),
      );

      event.dataTransfer!.clearData(DRAG_FORMAT);
      event.dataTransfer!.setData(DRAG_FORMAT, data);
      event.dataTransfer!.dropEffect = effect;
      event.dataTransfer!.setDragImage(image, 0, 0);
    }  else if (isMouseMove(event)) {
      this.mouse = [event.clientX, event.clientY];
    } else if (isKeyUp(event)) {
        const entity = this.shadowRoot
        .elementFromPoint(this.mouse[0], this.mouse[1])
        ?.closest("[id]");

      if (isFormInput(event.target)) {
        this.passEvent(IntentType.KeyUp, entity, {
          registers: [event.target.value, "", "", ""],
        });
      }
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
          this.passEvent(IntentType.EscPress, entity);
          break;
        case "ArrowDown":
          event.preventDefault();
          this.passEvent(IntentType.Scroll, entity, {
            registers: ["1", "", "", ""],
          });
          break;
        case "ArrowUp":
          event.preventDefault();
          this.passEvent(IntentType.Scroll, entity, {
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
            IntentType.ShowDebugMenu,
            this.shadowRoot.querySelector("[id]")!,
          );
        default:
          return;
      }
    } else if (isWheel(event)) {
      if (!this.mouseInCell) {
        return;
      }
      event.preventDefault();
      event.stopImmediatePropagation(); // prevent scroll capture by browser

      if (this.debounce) {
        return;
      }

      const direction = event.deltaY > 0 ? "10" : "-10";

      if (this.debounce) {
        return;
      } else {
        this.debounce = true;
        setTimeout(() => {
          this.debounce = false;
        }, DEBOUNCE_TIMEOUT);
      }

      const entity = this.shadowRoot
          .elementFromPoint(event.clientX, event.clientY)!
          .closest("[id]");
      if (!entity) {
        // if scroll does not happen in a part of the UI that is recorded
        return;
      }
      this.passEvent(IntentType.Scroll, entity, {
        registers: [direction, "", "", ""],
      });
    } else {
      console.log("Unknown event", event);
    }
  }
  /** mouseInCell is true if the pointer position is withing the boundary of the cell */
  get mouseInCell(): boolean {
    // global events, only accept if this is within the cell
    const vp = this.getBoundingClientRect();
    return (
        this.mouse[0] > vp.left &&
        this.mouse[0] < vp.right &&
        this.mouse[1] > vp.top &&
        this.mouse[1] < vp.bottom
    );
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

    // keeping this synchronous is required when we handle drag start event, where the payload and image must be found in the same loop.
    // [dnd] https://html.spec.whatwg.org/multipage/dnd.html#concept-dnd-rw
    queueMicrotask(sched);
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

  /*if (isSubmitButton(targetNode)) {
    if (!!targetNode.form) {
      return targetNode.form
    }
    return targetNode
  }*/

  const closest = targetNode?.closest("[id]");
  if (!closest) {
    return null;
  }
  return closest as HTMLElement;
}

/**
 * acceptDrop prevents the drag event propagation, optionally only if pred is provided.
 * This has the effect of allowing the drag event to happen.
 * Return whether the event was accepted or not
 */
function acceptDrop(e: DragEvent, pred?: (e: DragEvent) => boolean) {
  if (!pred || pred(e)) {
    e.preventDefault();
    e.stopImmediatePropagation();
    return true;
  }
  return false;
}

// Typescript checks with inference
const isClick = (ev: Event) => ev.type === "click";
const isDoubleClick = (ev: Event) =>
  ev.type === "dblclick" || (isClick(ev) && (ev as MouseEvent).detail == 2);
const isRightClick = (ev: Event) => ev.type === "contextmenu";
const isMouseMove = (ev: Event): ev is MouseEvent => ev.type === "mousemove";
const isWheel = (ev: Event): ev is WheelEvent => ev.type === "wheel";
const isKeyUp = (ev: Event): ev is KeyboardEvent => ev.type === "keyup";
const isKeyDown = (ev: Event): ev is KeyboardEvent => ev.type === "keydown";
const isDragStart = (ev: Event): ev is DragEvent => ev.type === "dragstart";
const isDragOver = (ev: Event): ev is DragEvent => ev.type === "dragover";
const isDragEnter = (ev: Event): ev is DragEvent => ev.type === "dragenter";
const isDrop = (ev: Event): ev is DragEvent => ev.type === "drop";
const isDragEnd = (ev: Event): ev is DragEvent => ev.type === "dragend";
const isSubmitButton = (
  target: EventTarget | null,
): target is HTMLInputElement =>
  target instanceof HTMLInputElement && target.type === "submit";

  const isFormInput = (
  target: EventTarget | null,
): target is HTMLInputElement =>
  target instanceof HTMLInputElement && (target.type === "text" || target.type === "password" || target.type === "date");

function assertsMouseEvent(ev: Event): asserts ev is MouseEvent {
  if (!(ev instanceof MouseEvent)) {
    throw new Error("event is not MouseEvent");
  }
}
