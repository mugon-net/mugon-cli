Mugon.start(async (ctx) => {
  console.log(`Hello world! Running as ${ctx.role} with id ${ctx.ownId}`);
  ctx.on("connect", (peerId) => console.log(`Peer connected: ${peerId}`));
});
