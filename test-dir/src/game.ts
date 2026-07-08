// Authoritative game simulation. Runs only on the host.

import { FoodDelta, FoodState, WirePlayer } from "./protocol";

export const WORLD = 3000;
export const FOOD_COUNT = 300;
export const FOOD_MASS = 1;
export const START_MASS = 12;

// A cell's on-screen radius grows with the square root of its mass, so mass
// (area) doubling only grows the radius by ~1.4x — the classic agar.io feel.
export function radiusOf(mass: number): number {
  return Math.sqrt(mass) * 6;
}

// Bigger cells move slower. Shared with the client so prediction matches the
// server exactly.
export function speedOf(mass: number): number {
  return 240 / Math.pow(mass, 0.22);
}

function randomPoint(): number {
  return Math.random() * WORLD;
}

function randomHue(): number {
  return Math.floor(Math.random() * 360);
}

function clamp(v: number, lo: number, hi: number): number {
  return v < lo ? lo : v > hi ? hi : v;
}

function distSq(ax: number, ay: number, bx: number, by: number): number {
  const dx = ax - bx;
  const dy = ay - by;
  return dx * dx + dy * dy;
}

type ServerPlayer = WirePlayer & { inputX: number; inputY: number };

export class Game {
  private playersById = new Map<string, ServerPlayer>();
  private food: FoodState[] = [];
  private changedFood = new Set<number>();
  private tickCount = 0;

  constructor() {
    for (let i = 0; i < FOOD_COUNT; i++) {
      this.food.push({ x: randomPoint(), y: randomPoint() });
    }
  }

  get currentTick(): number {
    return this.tickCount;
  }

  addPlayer(id: string): void {
    this.playersById.set(id, {
      id,
      x: randomPoint(),
      y: randomPoint(),
      mass: START_MASS,
      hue: randomHue(),
      inputX: 0,
      inputY: 0,
    });
  }

  removePlayer(id: string): void {
    this.playersById.delete(id);
  }

  setInput(id: string, x: number, y: number): void {
    const p = this.playersById.get(id);
    if (!p) return;
    p.inputX = x;
    p.inputY = y;
  }

  private replaceFood(i: number): void {
    this.food[i] = { x: randomPoint(), y: randomPoint() };
    this.changedFood.add(i);
  }

  private respawn(p: ServerPlayer): void {
    p.x = randomPoint();
    p.y = randomPoint();
    p.mass = START_MASS;
  }

  tick(dt: number): void {
    this.tickCount++;

    for (const p of this.playersById.values()) {
      const step = speedOf(p.mass) * dt;
      p.x = clamp(p.x + p.inputX * step, 0, WORLD);
      p.y = clamp(p.y + p.inputY * step, 0, WORLD);
    }

    for (const p of this.playersById.values()) {
      const rSq = radiusOf(p.mass) ** 2;
      for (let i = 0; i < this.food.length; i++) {
        const f = this.food[i];
        if (distSq(p.x, p.y, f.x, f.y) < rSq) {
          p.mass += FOOD_MASS;
          this.replaceFood(i);
        }
      }
    }

    const players = [...this.playersById.values()];
    for (const a of players) {
      for (const b of players) {
        if (a === b) continue;
        if (a.mass <= b.mass * 1.15) continue;
        if (distSq(a.x, a.y, b.x, b.y) < radiusOf(a.mass) ** 2) {
          a.mass += b.mass;
          this.respawn(b);
        }
      }
    }
  }

  // Fresh plain objects each call, so callers can retain snapshots without them
  // mutating on the next tick.
  players(): WirePlayer[] {
    return [...this.playersById.values()].map((p) => ({
      id: p.id,
      x: p.x,
      y: p.y,
      mass: p.mass,
      hue: p.hue,
    }));
  }

  foodList(): FoodState[] {
    return this.food;
  }

  takeChangedFood(): FoodDelta[] {
    const deltas: FoodDelta[] = [];
    for (const i of this.changedFood) {
      deltas.push({ index: i, x: this.food[i].x, y: this.food[i].y });
    }
    this.changedFood.clear();
    return deltas;
  }
}
