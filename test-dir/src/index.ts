import { getSettingValue, setupParentFrameCommunication } from "@mugon/sdk";

console.log("Gameframe initializing...");
setupParentFrameCommunication(
  (msg) => console.log("ready", msg),
  (msg) => console.log("connected", msg),
  (msg) => console.log("disconnected", msg),
  (msg) => console.log("data", msg),
  (msg) =>
    console.log(
      "settings",
      msg,
      getSettingValue("mugon.networkmode"),
    ),
);
