import { defineConfig } from 'vite';
import laravel from 'laravel-vite-plugin';
import vue from '@vitejs/plugin-vue';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
    plugins: [
        laravel({
            input: ['resources/css/app.css', 'resources/js/app.js'],
            refresh: true,
        }),
        vue(),
        tailwindcss(),
    ],
    server: {
        // 9050-range dev ports so a co-running Laravel app on :8000/:5173 doesn't
        // collide (artisan serve → :9050). If :9051 is taken Vite bumps to the next
        // free port and laravel-vite-plugin writes the real one to public/hot.
        port: 9051,
        watch: {
            ignored: ['**/storage/framework/views/**'],
        },
    },
});
