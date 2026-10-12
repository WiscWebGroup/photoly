/* All assets and API requests stay on this backend. Tokens live in memory only. */
window.ui = SwaggerUIBundle({
  url: document.currentScript.dataset.specUrl,
  dom_id: '#swagger-ui',
  presets: [SwaggerUIBundle.presets.apis],
  layout: 'BaseLayout',
  deepLinking: true,
  filter: true,
  persistAuthorization: false,
  validatorUrl: null,
  supportedSubmitMethods: ['get', 'post'],
  defaultModelsExpandDepth: 0,
});
