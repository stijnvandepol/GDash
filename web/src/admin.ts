// Entry point for the web interface at "/".
import { mount } from 'svelte';
import Admin from './admin/Admin.svelte';

mount(Admin, { target: document.getElementById('app')! });
