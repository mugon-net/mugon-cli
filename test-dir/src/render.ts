// Renders a world view with a camera that follows the local player and zooms
// out as it grows.

import { FoodState, WirePlayer } from "./protocol";
import { radiusOf, WORLD } from "./game";

const TAU = Math.PI * 2;

export type RenderView = {
  players: WirePlayer[];
  food: FoodState[];
};

function clamp(v: number, lo: number, hi: number): number {
  return v < lo ? lo : v > hi ? hi : v;
}

export function render(
  ctx: CanvasRenderingContext2D,
  canvas: HTMLCanvasElement,
  view: RenderView,
  myId: string,
): void {
  const w = canvas.width;
  const h = canvas.height;

  ctx.fillStyle = "#0e0e12";
  ctx.fillRect(0, 0, w, h);

  const me = view.players.find((p) => p.id === myId);
  const camX = me ? me.x : WORLD / 2;
  const camY = me ? me.y : WORLD / 2;
  const myR = me ? radiusOf(me.mass) : 40;
  const scale = clamp(Math.min(w, h) / (myR * 14), 0.15, 2.2);

  ctx.save();
  ctx.translate(w / 2, h / 2);
  ctx.scale(scale, scale);
  ctx.translate(-camX, -camY);

  drawGrid(ctx);

  ctx.lineWidth = 6;
  ctx.strokeStyle = "#2a2a35";
  ctx.strokeRect(0, 0, WORLD, WORLD);

  ctx.fillStyle = "#5ec8ff";
  for (const f of view.food) {
    if (!f) continue;
    ctx.beginPath();
    ctx.arc(f.x, f.y, 7, 0, TAU);
    ctx.fill();
  }

  // Draw smallest first so larger cells overlap them.
  const players = [...view.players].sort((a, b) => a.mass - b.mass);
  for (const p of players) {
    const r = radiusOf(p.mass);
    ctx.beginPath();
    ctx.arc(p.x, p.y, r, 0, TAU);
    ctx.fillStyle = `hsl(${p.hue} 70% 55%)`;
    ctx.fill();
    ctx.lineWidth = p.id === myId ? 5 : 3;
    ctx.strokeStyle = p.id === myId ? "#ffffff" : "rgba(0,0,0,0.35)";
    ctx.stroke();
  }

  ctx.restore();

  if (me) {
    ctx.fillStyle = "#ffffff";
    ctx.font = "20px sans-serif";
    ctx.fillText(`Mass: ${Math.floor(me.mass)}`, 16, 30);
    ctx.fillText(`Players: ${view.players.length}`, 16, 56);
  }
}

function drawGrid(ctx: CanvasRenderingContext2D): void {
  ctx.strokeStyle = "#17171f";
  ctx.lineWidth = 1;
  ctx.beginPath();
  for (let x = 0; x <= WORLD; x += 100) {
    ctx.moveTo(x, 0);
    ctx.lineTo(x, WORLD);
  }
  for (let y = 0; y <= WORLD; y += 100) {
    ctx.moveTo(0, y);
    ctx.lineTo(WORLD, y);
  }
  ctx.stroke();
}

export function drawWaiting(
  ctx: CanvasRenderingContext2D,
  canvas: HTMLCanvasElement,
): void {
  ctx.fillStyle = "#0e0e12";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = "#ffffff";
  ctx.font = "24px sans-serif";
  ctx.textAlign = "center";
  ctx.fillText(
    "Click “Start as host” or “Start as client” below to play",
    canvas.width / 2,
    canvas.height / 2,
  );
  ctx.textAlign = "left";
}
