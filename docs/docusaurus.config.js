// @ts-check
/** @type {import('@docusaurus/types').Config} */
module.exports = {
  title: 'Photoly',
  tagline: 'Your photos, on your terms',
  url: process.env.DOCS_URL || 'https://photoly-docs.pages.dev',
  baseUrl: '/',
  organizationName: 'WiscWebGroup',
  projectName: 'photoly',
  onBrokenLinks: 'throw',
  // The existing docs are plain CommonMark, not MDX.
  markdown: { format: 'detect', hooks: { onBrokenMarkdownLinks: 'throw' } },
  presets: [
    [
      'classic',
      {
        // Docs-only mode: overview/welcome.md (slug: /) is the landing page.
        docs: {
          routeBasePath: '/',
          sidebarPath: './sidebars.js',
          editUrl: 'https://github.com/WiscWebGroup/photoly/tree/main/docs/',
        },
        blog: false,
        theme: {},
      },
    ],
  ],
  themeConfig: {
    navbar: {
      title: 'Photoly',
      items: [
        { to: '/', label: 'Docs', position: 'left' },
        { to: '/api', label: 'API', position: 'left' },
        { href: 'https://github.com/WiscWebGroup/photoly', label: 'GitHub', position: 'right' },
      ],
    },
  },
};
