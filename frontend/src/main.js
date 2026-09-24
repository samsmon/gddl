import { mount } from 'svelte'
import './app.css'
import './styles/layout.css'
import './styles/table.css'
import './styles/modals.css'
import './styles/auth_oauth.css'
import './styles/docs_logs.css'
import App from './App.svelte'

const app = mount(App, {
  target: document.getElementById('app'),
})

export default app
