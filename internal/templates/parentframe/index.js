function init() {
  const channel = new MessageChannel();

  const iframe = document.querySelector('iframe');
  iframe.addEventListener('load', () => {
    iframe.contentWindow.postMessage({ type: 'init' }, '*', [channel.port2]);
  });
  channel.port1.onmessage = handleChildFrameMessage;
  console.log("Parentframe initialized.")
}

function handleChildFrameMessage(event) {
  const msg = event.data;

  switch(event.data.type) {
    case "rdy":
      const protocol = event.data.protocol;
      // TODO
      break;
    case "msg":
      const toclientid = event.data.toclientid;
      const data = event.data.data;
      // TODO
      break;
    case "drp":
      const clientid = event.data.clientid;
      // TODO
      break;
  }
}

init();