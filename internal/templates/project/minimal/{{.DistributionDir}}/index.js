Mugon.start(async (ctx) => {
  console.log(`Hello world! Running as ${ctx.role} with id ${ctx.self.id}`);
  ctx.on("playerConnected", (player) => console.log(`Player connected: ${player.name}`));
});
