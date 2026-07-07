import { FromGameframeMessage, PROTOCOL } from "@mugon/sdk";

let port: MessagePort | undefined;

const settings: Record<string, string | undefined> = {
  "mugon.networkmode": undefined,
};

function init() {
  const channel = new MessageChannel();
  port = channel.port1;

  const iframe = document.querySelector("iframe")!;
  iframe.addEventListener("load", () => {
    iframe.contentWindow!.postMessage({ type: "init" }, "*", [channel.port2]);
  });
  port.onmessage = handleChildFrameMessage;

  const startServerButton = document.getElementById(
    "start-server-button",
  ) as HTMLButtonElement;
  const joinServerButton = document.getElementById(
    "join-server-button",
  ) as HTMLButtonElement;

  startServerButton.addEventListener("click", () => {
    settings["mugon.networkmode"] = "server";
    startServerButton.disabled = true;
    joinServerButton.disabled = true;
    sendSettings();
    setTimeout(sendReady, 250);
  });
  joinServerButton.addEventListener("click", () => {
    settings["mugon.networkmode"] = "client";
    startServerButton.disabled = true;
    joinServerButton.disabled = true;
    sendSettings();
    setTimeout(sendReady, 250);
  });

  console.log("Parentframe initialized.");
}

function handleChildFrameMessage(event: MessageEvent<FromGameframeMessage>) {
  const msg = event.data;

  switch (msg.type) {
    case "rdy":
      if (msg.protocol !== PROTOCOL) {
        alert(
          `Mismatching protocol version: CLI protocol version: '${PROTOCOL}'; Game protocol version: '${msg.protocol}'`,
        );
      }
      port!.onmessage = null;
      break;
    case "msg":
      // TODO
      break;
    case "drp":
      // TODO
      break;
  }
}

function sendSettings() {
  port!.postMessage({ type: "set", settings });
}

function sendReady() {
  port!.postMessage({ type: "rdy", protocol: PROTOCOL });
}

init();
