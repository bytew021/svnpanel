import { createApp } from "vue";
import { createPinia } from "pinia";
import VxeUI from "vxe-pc-ui";
import VxeUITable from "vxe-table";
import "vxe-pc-ui/lib/style.css";
import "vxe-table/lib/style.css";
import App from "./App.vue";
import router from "./router";
import "./style.css";

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(VxeUI);
app.use(VxeUITable);
VxeUI.setConfig({ select: { transfer: true } });
app.mount("#app");
