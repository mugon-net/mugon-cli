// The SDK must be imported first: importing it says hello to the platform.
import { Mugon } from "@mugon/sdk";

Mugon.start(async (ctx) => {
  console.log(`Hello world! Running as ${ctx.role} with id ${ctx.self.id}`);

  ctx.on("playerConnected", (player) =>
    console.log(`Player connected: ${player.name}`),
  );
  ctx.on("message", ({ from, data }) =>
    console.log(`Message from ${from}: ${data.byteLength} bytes`),
  );
});
