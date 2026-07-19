import {
  Channel,
  getSettingValue,
  sendDataMessage,
  setupParentFrameCommunication,
  startCanvasManager,
} from "@mugon/sdk";
import { Game, speedOf, WORLD } from "./game";
import {
  decodeFoodDelta,
  decodeFullFood,
  decodeInput,
  decodeState,
  encodeFoodDelta,
  encodeFullFood,
  encodeInput,
  encodeState,
  FoodState,
  messageType,
  MSG_FOOD_DELTA,
  MSG_FULL_FOOD,
  MSG_INPUT,
  MSG_STATE,
  WirePlayer,
} from "./protocol";
import { drawWaiting, render, RenderView } from "./render";

const canvas = document.getElementById("canvas") as HTMLCanvasElement;
const ctx = canvas.getContext("2d")!;

const TICK_MS = 1000 / 30;
// Render the world this far in the past so we always have two snapshots to
// interpolate between, hiding network jitter.
const INTERP_DELAY_MS = 100;
// Per-frame fraction by which the predicted local position is pulled toward the
// authoritative one, smoothing out any divergence.
const RECONCILE = 0.15;

let role: "server" | "client" | undefined;
let myId = "";
let serverId = "";

// Host-only state.
let game: Game | undefined;
const clients = new Set<string>();

// Rendering state, shared by host and client. The host feeds its own
// simulation into the same buffer the client fills from the network, so both
// render through one interpolating path.
type BufferedSnapshot = { time: number; players: WirePlayer[] };
const snapBuffer: BufferedSnapshot[] = [];
let food: FoodState[] = [];

// Local desired movement direction, magnitude in [0, 1]. Seeded with a random
// heading so the player is moving from the start, even on touch devices where
// there is no cursor position to read yet.
const startAngle = Math.random() * Math.PI * 2;
const input = { x: Math.cos(startAngle), y: Math.sin(startAngle) };

// Client-side prediction of the local player's position.
let predicted: { x: number; y: number } | undefined;

setupParentFrameCommunication(
  undefined,
  (msg) => {
    if (role !== "server" || !game) return;
    clients.add(msg.clientid);
    game.addPlayer(msg.clientid);
    sendDataMessage(
      msg.clientid,
      encodeFullFood(game.foodList()),
      Channel.ReliableOrdered,
    );
  },
  (msg) => {
    if (role !== "server" || !game) return;
    clients.delete(msg.clientid);
    game.removePlayer(msg.clientid);
  },
  (msg) => {
    const type = messageType(msg.data);
    if (role === "server" && type === MSG_INPUT) {
      const dir = decodeInput(msg.data);
      game?.setInput(msg.fromclientid, dir.x, dir.y);
    } else if (role === "client" && type === MSG_STATE) {
      pushSnapshot(decodeState(msg.data).players);
    } else if (role === "client" && type === MSG_FOOD_DELTA) {
      for (const d of decodeFoodDelta(msg.data)) food[d.index] = { x: d.x, y: d.y };
    } else if (role === "client" && type === MSG_FULL_FOOD) {
      food = decodeFullFood(msg.data);
    }
  },
  () => {
    if (role) return;
    const mode = getSettingValue("mugon.networkmode");
    if (mode === "server") {
      role = "server";
      myId = getSettingValue("mugon.serverid")!;
      game = new Game();
      game.addPlayer(myId);
      food = game.foodList();
      startHostLoop();
    } else if (mode === "client") {
      role = "client";
      myId = getSettingValue("mugon.clientid")!;
      serverId = getSettingValue("mugon.serverid")!;
      startClientLoop();
    }
  },
);

function startHostLoop(): void {
  setInterval(() => {
    if (!game) return;
    game.setInput(myId, input.x, input.y);
    game.tick(TICK_MS / 1000);

    const players = game.players();
    pushSnapshot(players);
    const state = encodeState(game.currentTick, players);

    const changed = game.takeChangedFood();
    const foodDelta = changed.length > 0 ? encodeFoodDelta(changed) : undefined;

    // Buffers are transferred on send (detaching them), so hand each client its
    // own copy. State is disposable -> unreliable. Food deltas must arrive, and
    // must ride the same reliable *ordered* channel as the join-time full-food
    // snapshot: on a separate channel a delta can overtake the snapshot and be
    // clobbered when the snapshot lands, leaving the client permanently missing
    // that pellet.
    for (const clientId of clients) {
      sendDataMessage(clientId, state.slice(), Channel.UnreliableOrdered);
      if (foodDelta) {
        sendDataMessage(clientId, foodDelta.slice(), Channel.ReliableOrdered);
      }
    }
  }, TICK_MS);
}

function startClientLoop(): void {
  setInterval(() => {
    sendDataMessage(
      serverId,
      encodeInput(input.x, input.y),
      Channel.UnreliableOrdered,
    );
  }, TICK_MS);
}

function pushSnapshot(players: WirePlayer[]): void {
  snapBuffer.push({ time: performance.now(), players });
  const cutoff = performance.now() - 1000;
  while (snapBuffer.length > 2 && snapBuffer[0].time < cutoff) {
    snapBuffer.shift();
  }
}

// Position/mass of the local player from the most recent snapshot.
function latestSelf(): WirePlayer | undefined {
  for (let i = snapBuffer.length - 1; i >= 0; i--) {
    const self = snapBuffer[i].players.find((p) => p.id === myId);
    if (self) return self;
  }
  return undefined;
}

// Interpolated positions of every player at the given render time.
function sampleAt(time: number): Map<string, WirePlayer> {
  const result = new Map<string, WirePlayer>();
  if (snapBuffer.length === 0) return result;

  const first = snapBuffer[0];
  const last = snapBuffer[snapBuffer.length - 1];
  if (snapBuffer.length === 1 || time <= first.time) {
    for (const p of first.players) result.set(p.id, p);
    return result;
  }
  if (time >= last.time) {
    for (const p of last.players) result.set(p.id, p);
    return result;
  }

  let i = 0;
  while (i < snapBuffer.length - 1 && snapBuffer[i + 1].time < time) i++;
  const a = snapBuffer[i];
  const b = snapBuffer[i + 1];
  const t = (time - a.time) / (b.time - a.time);
  const bById = new Map(b.players.map((p) => [p.id, p]));

  for (const pa of a.players) {
    const pb = bById.get(pa.id);
    if (pb) {
      result.set(pa.id, {
        id: pa.id,
        hue: pa.hue,
        x: pa.x + (pb.x - pa.x) * t,
        y: pa.y + (pb.y - pa.y) * t,
        mass: pa.mass + (pb.mass - pa.mass) * t,
      });
    } else {
      result.set(pa.id, pa);
    }
  }
  for (const pb of b.players) {
    if (!result.has(pb.id)) result.set(pb.id, pb);
  }
  return result;
}

function updatePrediction(dt: number): WirePlayer | undefined {
  const self = latestSelf();
  if (!self) return undefined;
  if (!predicted) predicted = { x: self.x, y: self.y };

  const step = speedOf(self.mass) * dt;
  predicted.x = clamp(predicted.x + input.x * step, 0, WORLD);
  predicted.y = clamp(predicted.y + input.y * step, 0, WORLD);
  predicted.x += (self.x - predicted.x) * RECONCILE;
  predicted.y += (self.y - predicted.y) * RECONCILE;

  return { id: myId, x: predicted.x, y: predicted.y, mass: self.mass, hue: self.hue };
}

function setupInput(): void {
  const deadzone = 120;

  // Aim toward a point on screen; magnitude scales with distance from centre so
  // the pointer near the middle slows you down. Works for both a mouse cursor
  // and a dragged finger.
  const aimAt = (clientX: number, clientY: number): void => {
    const dx = clientX - canvas.clientWidth / 2;
    const dy = clientY - canvas.clientHeight / 2;
    const len = Math.hypot(dx, dy);
    if (len < 1) {
      input.x = 0;
      input.y = 0;
      return;
    }
    const speed = Math.min(len, deadzone) / deadzone;
    input.x = (dx / len) * speed;
    input.y = (dy / len) * speed;
  };

  window.addEventListener("mousemove", (e) => aimAt(e.clientX, e.clientY));

  const onTouch = (e: TouchEvent): void => {
    const touch = e.touches[0];
    if (touch) aimAt(touch.clientX, touch.clientY);
    // Stop the browser from scrolling / pull-to-refreshing while steering.
    e.preventDefault();
  };
  window.addEventListener("touchstart", onTouch, { passive: false });
  window.addEventListener("touchmove", onTouch, { passive: false });
  // Lifting the finger keeps the last heading, mirroring how a mouse cursor
  // stays where it was left.
}

let lastFrame = performance.now();

function renderLoop(now: number): void {
  const dt = Math.min((now - lastFrame) / 1000, 0.1);
  lastFrame = now;

  if (snapBuffer.length === 0) {
    drawWaiting(ctx, canvas);
    requestAnimationFrame(renderLoop);
    return;
  }

  const self = updatePrediction(dt);
  const sampled = sampleAt(now - INTERP_DELAY_MS);
  if (self) sampled.set(myId, self);

  const view: RenderView = { players: [...sampled.values()], food };
  render(ctx, canvas, view, myId);
  requestAnimationFrame(renderLoop);
}

function clamp(v: number, lo: number, hi: number): number {
  return v < lo ? lo : v > hi ? hi : v;
}

startCanvasManager();
setupInput();
requestAnimationFrame(renderLoop);
