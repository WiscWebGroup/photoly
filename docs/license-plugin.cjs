const path = require('node:path')
const fs = require('node:fs')
module.exports = function () {
  return {
    name: 'photoly-license-check',
    configureWebpack(_config, isServer) {
      return { plugins: [{ apply(compiler) {
        compiler.hooks.afterEmit.tapPromise('PhotolyLicenseCheck', async (compilation) => {
          const { validateBundle } = await import('../photoly_frontend_vue/scripts/license-bundle.mjs')
          const files = []
          function visit(m) { if (m.resource) files.push(m.resource); for (const child of m.modules ?? []) visit(child) }
          for (const m of compilation.modules) visit(m)
          const packages = validateBundle(__dirname, files)
          const dir = path.join(compiler.options.output.path, 'legal')
          fs.mkdirSync(dir, { recursive: true })
          fs.writeFileSync(path.join(dir, isServer ? 'server-build-dependencies.json' : 'bundled-dependencies.json'), JSON.stringify({ packages }, null, 2) + '\n')
        })
      } }] }
    },
  }
}
