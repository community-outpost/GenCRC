// Minimal DOM harness for the page's event and state contract. This does not
// claim browser layout, native picker, or accessibility-tree verification.
export class Element {
  constructor() {
    this.children = []; this.listeners = {}; this.files = []; this.hidden = false;
    this.dataset = {}; this.value = ''; this.textContent = ''; this.clicks = 0;
    const classes = new Set();
    this.classList = { add: x => classes.add(x), remove: x => classes.delete(x),
      contains: x => classes.has(x), toggle: (x, yes) => yes ? classes.add(x) : classes.delete(x) };
  }
  addEventListener(name, callback) { (this.listeners[name] ||= []).push(callback); }
  async dispatch(name, event = {}) {
    event.preventDefault ||= () => {};
    event.target ||= this;
    if (this['on' + name]) await this['on' + name](event);
    for (const callback of this.listeners[name] || []) await callback(event);
  }
  click() { this.clicks++; return this.dispatch('click'); }
  append(...children) { this.children.push(...children); }
  replaceChildren(...children) { this.children = children; }
  setAttribute() {}
  focus() { this.focused = true; }
  scrollIntoView() {}
}
