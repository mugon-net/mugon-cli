// The SDK must be imported first: importing it says hello to the platform and starts file serving.
import { Mugon } from "@mugon/sdk";
import init from "../pkg/game.js";

Mugon.start(async () => {
  try {
    // Downloads game_bg.wasm through the platform's file proxy and runs `main`.
    await init();
  } catch (error) {
    // Bevy's web event loop unwinds `main` with this exception on purpose.
    if (!String(error).includes("control flow")) throw error;
  }
  // Keep the callback pending: it finishing would close the platform's loading overlay before
  // the game has loaded its assets. The game closes it itself (`Bridge::loaded`).
  await new Promise(() => {});
});
