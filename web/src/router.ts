import { createRouter, createWebHashHistory } from "vue-router";
import ConnectedApp from "./connected/ConnectedApp.vue";
import PreviewLayout from "./preview/PreviewLayout.vue";
import RecordList from "./preview/RecordList.vue";
import RecordDetail from "./preview/RecordDetail.vue";
import RecordForm from "./preview/RecordForm.vue";
import EvidencePage from "./preview/EvidencePage.vue";
import NotFound from "./preview/NotFound.vue";
const objectRoutes = (["accounts", "contacts"] as const).flatMap((object) => [
  { path: object, component: RecordList, props: { object } },
  { path: `${object}/new`, component: RecordForm, props: { object } },
  { path: `${object}/:id/edit`, component: RecordForm, props: { object } },
  { path: `${object}/:id`, component: RecordDetail, props: { object } },
]);
export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: "/", component: ConnectedApp },
    {
      path: "/preview",
      component: PreviewLayout,
      children: [
        { path: "", redirect: "/preview/accounts" },
        ...objectRoutes,
        { path: "evidence", component: EvidencePage },
        { path: ":pathMatch(.*)*", component: NotFound },
      ],
    },
    {
      path: "/:pathMatch(.*)*",
      component: PreviewLayout,
      children: [{ path: "", component: NotFound }],
    },
  ],
  scrollBehavior: () => ({ top: 0 }),
});
