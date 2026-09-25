import { mount } from 'svelte'
import '@mdi/font/css/materialdesignicons.css'
import './lib/i18n'
import './app.css'
import App from './App.svelte'

const app = mount(App, {
  target: document.getElementById('app')!,
})

export default app
