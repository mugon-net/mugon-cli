import { createRoot } from "react-dom/client";
import { App, MugonConfig } from "./App";

declare global {
  interface Window {
    __MUGON_CONFIG__?: MugonConfig;
  }
}

const config = window.__MUGON_CONFIG__;
const root = document.getElementById("root");

if (!config) {
  alert("mugon dev config was not injected into the page");
} else if (root) {
  createRoot(root).render(<App config={config} />);
}
