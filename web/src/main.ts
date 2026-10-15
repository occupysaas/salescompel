import { createApp } from "vue";
import App from "./App.vue";
import { setLanguage } from "./i18n";
import "./style.css";
await setLanguage("en_US");
createApp(App).mount("#app");
