// Display names: a colour word chosen to match the blob's hue, plus an animal
// picked deterministically from the player id so every client shows the same
// name for the same player without any extra network traffic.

const COLORS: { name: string; maxHue: number }[] = [
  { name: "Red", maxHue: 15 },
  { name: "Orange", maxHue: 45 },
  { name: "Yellow", maxHue: 70 },
  { name: "Lime", maxHue: 95 },
  { name: "Green", maxHue: 150 },
  { name: "Teal", maxHue: 185 },
  { name: "Cyan", maxHue: 205 },
  { name: "Blue", maxHue: 255 },
  { name: "Indigo", maxHue: 275 },
  { name: "Purple", maxHue: 300 },
  { name: "Magenta", maxHue: 330 },
  { name: "Pink", maxHue: 348 },
  { name: "Red", maxHue: 360 },
];

const ANIMALS = [
  "Otter", "Falcon", "Badger", "Lynx", "Heron", "Marten", "Bison", "Panther",
  "Gecko", "Walrus", "Ferret", "Osprey", "Beaver", "Cobra", "Mantis", "Puffin",
  "Jackal", "Weasel", "Raven", "Newt", "Stoat", "Quokka", "Tapir", "Ibex",
  "Wombat", "Shrew", "Vole", "Kudu", "Gannet", "Meerkat", "Civet", "Serval",
];

export function colorName(hue: number): string {
  const h = ((hue % 360) + 360) % 360;
  for (const c of COLORS) {
    if (h <= c.maxHue) return c.name;
  }
  return COLORS[COLORS.length - 1].name;
}

function hashId(id: string): number {
  let h = 2166136261;
  for (let i = 0; i < id.length; i++) {
    h ^= id.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}

export function animalName(id: string): string {
  return ANIMALS[hashId(id) % ANIMALS.length];
}

export function playerName(id: string, hue: number): string {
  return `${colorName(hue)} ${animalName(id)}`;
}
