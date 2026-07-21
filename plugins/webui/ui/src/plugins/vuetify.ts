import '@mdi/font/css/materialdesignicons.css'
import 'vuetify/styles'

import { createVuetify } from 'vuetify'

export const vuetify = createVuetify({
  theme: {
    defaultTheme: 'light',
    themes: {
      light: {
        colors: {
          background: '#f5f7fb',
          surface: '#ffffff',
          primary: '#1f8aa8',
          secondary: '#16324f',
          success: '#36a56a',
          warning: '#e3a008',
        },
      },
    },
  },
})
