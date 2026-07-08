// Binary wire format for the mugon data channel. Kept compact so a 30 Hz
// snapshot stream stays cheap: player data is small and fixed-size, and food is
// sent in full only once (on join) and then as per-tick deltas.
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
export const MSG_SNAPSHOT = 1;
export const MSG_FULL_FOOD = 2;

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

// Host -> client: tick number, all players, and only the food that changed this
// tick.
export function encodeSnapshot(
  tick: number,
  players: WirePlayer[],
  deltas: FoodDelta[],
): Uint8Array {
  const ids = players.map((p) => encoder.encode(p.id));
  let size = 1 + 4 + 2;
  for (const id of ids) size += 1 + id.length + 4 + 4 + 4 + 2;
  size += 2 + deltas.length * (2 + 4 + 4);

  const buf = new Uint8Array(size);
  const dv = new DataView(buf.buffer);
  let o = 0;
  dv.setUint8(o, MSG_SNAPSHOT);
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

export function decodeSnapshot(data: Uint8Array): {
  tick: number;
  players: WirePlayer[];
  deltas: FoodDelta[];
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
  const deltaCount = dv.getUint16(o);
  o += 2;
  const deltas: FoodDelta[] = [];
  for (let i = 0; i < deltaCount; i++) {
    const index = dv.getUint16(o);
    o += 2;
    const x = dv.getFloat32(o);
    o += 4;
    const y = dv.getFloat32(o);
    o += 4;
    deltas.push({ index, x, y });
  }
  return { tick, players, deltas };
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
