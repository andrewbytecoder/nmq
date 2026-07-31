import { createApp } from 'vue'
import { VueFinderPlugin } from 'vuefinder'

import App from './App.vue'
import { vuetify } from './plugins/vuetify'
import { router } from './router'
import './styles/main.css'

const app = createApp(App)

app.use(router)
app.use(vuetify)
app.use(VueFinderPlugin, {
  locale: 'zhCN',
})
app.mount('#app')
