// Entry point for the web interface at "/".
import '@fontsource-variable/atkinson-hyperlegible-next';
import '@fontsource-variable/fraunces';
import { mount } from 'svelte';
import Admin from './admin/Admin.svelte';

mount(Admin, { target: document.getElementById('app')! });
