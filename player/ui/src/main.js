import { mount } from 'svelte';
import App from './App.svelte';
import './osd.css';

export default mount(App, { target: document.getElementById('osd') });
