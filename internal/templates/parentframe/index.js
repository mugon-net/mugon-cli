const channel = new MessageChannel();

const iframe = document.querySelector('iframe');
iframe.addEventListener('load', () => {
  iframe.contentWindow.postMessage({ type: 'init' }, '*', [channel.port2]);
});
channel.port1.onmessage = (e) => console.log('from child:', e.data);
channel.port1.postMessage({ hello: 'child' });

console.log("Hello world :)")