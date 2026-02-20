// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	site: 'https://boilerplate.gruntwork.io',
	integrations: [
		starlight({
			title: 'Gruntwork Boilerplate',
			logo: {
				light: './src/assets/boilerplate_logo_dark.svg',
				dark: './src/assets/boilerplate_logo_light.svg',
				replacesTitle: true,
			},
			description: 'Documentation and guides for Gruntwork Boilerplate',
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/gruntwork-io/boilerplate' }],
			customCss: [
				'./src/styles/custom.css',
			],
			defaultLocale: 'root',
			locales: {
				root: {
					label: 'English',
					lang: 'en',
				},
			},
			components: {
				Head: './src/components/Head.astro',
				Footer: './src/components/Footer.astro',
			},
			head: [
				{
					tag: 'meta',
					attrs: {
						property: 'og:title',
						content: 'Gruntwork Boilerplate Documentation',
					},
				},
			],
			editLink: {
				baseUrl: 'https://github.com/gruntwork-io/boilerplate/edit/main/docs/',
			},
			sidebar: [
				{
					label: 'Intro',
					autogenerate: { directory: 'intro' },
				},
				{
					label: 'Configuration',
					autogenerate: { directory: 'configuration' },
					collapsed: true,
				},
				{
					label: 'Template Syntax',
					autogenerate: { directory: 'template-syntax' },
					collapsed: true,
				},
				{
					label: 'CLI Reference',
					autogenerate: { directory: 'cli' },
					collapsed: true,
				},
				{
					label: 'Advanced',
					autogenerate: { directory: 'advanced' },
					collapsed: true,
				},
			],
		}),
	],
});
