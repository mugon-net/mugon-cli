import { setupParentFrameCommunication } from "@mugon/sdk";

console.log("Hello world :) from game frame");
setupParentFrameCommunication(
  (msg) => console.log("ready", msg),
  (msg) => console.log("connected", msg),
  (msg) => console.log("disconnected", msg),
  (msg) => console.log("data", msg),
);
