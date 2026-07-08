// Binary wire format for the mugon data channel. Kept compact so a 30 Hz
// stream stays cheap.
//
// Messages are split by delivery need (see index.ts for the channel each is
// sent on):
//   - player state is disposable: a dropped snapshot is replaced by the next
//     tick, so it rides an unreliable channel.
//   - food changes are not disposable: a lost delta would leave a pellet
//     desynced forever, so full food and food deltas ride reliable channels.
//
// All multi-byte values are big-endian (DataView default), consistent on both
// ends.

export type WirePlayer = {
  id: string;
  x: number;
  y: number;
  mass: number;
  hue: number;
};

export type FoodState = { x: number; y: number };
export type FoodDelta = { index: number; x: number; y: number };

export const MSG_INPUT = 0;
export const MSG_STATE = 1;
export const MSG_FOOD_DELTA = 2;
export const MSG_FULL_FOOD = 3;

const encoder = new TextEncoder();
const decoder = new TextDecoder();

export function messageType(data: Uint8Array): number {
  return data[0];
}

// Client -> host: movement direction, each axis quantised to a signed byte.
export function encodeInput(x: number, y: number): Uint8Array {
  const buf = new Uint8Array(3);
  buf[0] = MSG_INPUT;
  const dv = new DataView(buf.buffer);
  dv.setInt8(1, quantise(x));
  dv.setInt8(2, quantise(y));
  return buf;
}

export function decodeInput(data: Uint8Array): { x: number; y: number } {
  const dv = view(data);
  return { x: dv.getInt8(1) / 127, y: dv.getInt8(2) / 127 };
}

// Host -> client: tick number and every player's position/mass.
export function encodeState(tick: number, players: WirePlayer[]): Uint8Array {
  const ids = players.map((p) => encoder.encode(p.id));
  let size = 1 + 4 + 2;
  for (const id of ids) size += 1 + id.length + 4 + 4 + 4 + 2;

  const buf = new Uint8Array(size);
  const dv = new DataView(buf.buffer);
  let o = 0;
  dv.setUint8(o, MSG_STATE);
  o += 1;
  dv.setUint32(o, tick);
  o += 4;
  dv.setUint16(o, players.length);
  o += 2;
  for (let i = 0; i < players.length; i++) {
    const p = players[i];
    const id = ids[i];
    dv.setUint8(o, id.length);
    o += 1;
    buf.set(id, o);
    o += id.length;
    dv.setFloat32(o, p.x);
    o += 4;
    dv.setFloat32(o, p.y);
    o += 4;
    dv.setFloat32(o, p.mass);
    o += 4;
    dv.setUint16(o, p.hue);
    o += 2;
  }
  return buf;
}

export function decodeState(data: Uint8Array): {
  tick: number;
  players: WirePlayer[];
} {
  const dv = view(data);
  let o = 1;
  const tick = dv.getUint32(o);
  o += 4;
  const playerCount = dv.getUint16(o);
  o += 2;
  const players: WirePlayer[] = [];
  for (let i = 0; i < playerCount; i++) {
    const idLen = dv.getUint8(o);
    o += 1;
    const id = decoder.decode(
      new Uint8Array(data.buffer, data.byteOffset + o, idLen),
    );
    o += idLen;
    const x = dv.getFloat32(o);
    o += 4;
    const y = dv.getFloat32(o);
    o += 4;
    const mass = dv.getFloat32(o);
    o += 4;
    const hue = dv.getUint16(o);
    o += 2;
    players.push({ id, x, y, mass, hue });
  }
  return { tick, players };
}

// Host -> client: the food eaten this tick, with its new position.
export function encodeFoodDelta(deltas: FoodDelta[]): Uint8Array {
  const buf = new Uint8Array(1 + 2 + deltas.length * (2 + 4 + 4));
  const dv = new DataView(buf.buffer);
  let o = 0;
  dv.setUint8(o, MSG_FOOD_DELTA);
  o += 1;
  dv.setUint16(o, deltas.length);
  o += 2;
  for (const d of deltas) {
    dv.setUint16(o, d.index);
    o += 2;
    dv.setFloat32(o, d.x);
    o += 4;
    dv.setFloat32(o, d.y);
    o += 4;
  }
  return buf;
}

export function decodeFoodDelta(data: Uint8Array): FoodDelta[] {
  const dv = view(data);
  let o = 1;
  const count = dv.getUint16(o);
  o += 2;
  const deltas: FoodDelta[] = [];
  for (let i = 0; i < count; i++) {
    const index = dv.getUint16(o);
    o += 2;
    const x = dv.getFloat32(o);
    o += 4;
    const y = dv.getFloat32(o);
    o += 4;
    deltas.push({ index, x, y });
  }
  return deltas;
}

// Host -> client on join: the whole food field once.
export function encodeFullFood(food: FoodState[]): Uint8Array {
  const buf = new Uint8Array(1 + 2 + food.length * (4 + 4));
  const dv = new DataView(buf.buffer);
  let o = 0;
  dv.setUint8(o, MSG_FULL_FOOD);
  o += 1;
  dv.setUint16(o, food.length);
  o += 2;
  for (const f of food) {
    dv.setFloat32(o, f.x);
    o += 4;
    dv.setFloat32(o, f.y);
    o += 4;
  }
  return buf;
}

export function decodeFullFood(data: Uint8Array): FoodState[] {
  const dv = view(data);
  let o = 1;
  const count = dv.getUint16(o);
  o += 2;
  const food: FoodState[] = [];
  for (let i = 0; i < count; i++) {
    const x = dv.getFloat32(o);
    o += 4;
    const y = dv.getFloat32(o);
    o += 4;
    food.push({ x, y });
  }
  return food;
}

function quantise(v: number): number {
  return Math.max(-127, Math.min(127, Math.round(v * 127)));
}

function view(data: Uint8Array): DataView {
  return new DataView(data.buffer, data.byteOffset, data.byteLength);
}
