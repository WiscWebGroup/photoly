import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createHash } from 'node:crypto'
const root = new URL('../../', import.meta.url)
const read = (file) => readFileSync(new URL(file, root), 'utf8')

test('project license copies and model links match their authoritative sources', () => {
  const license = read('LICENSE')
  assert.equal(read('photoly_frontend_vue/licenses/AGPL-3.0.txt'), license)
  assert.equal(read('photoly_server/internal/legal/data/AGPL-3.0.txt'), license)
  const page = read('photoly_frontend_vue/src/views/LegalView.vue')
  const model = read('photoly_server/internal/ai/model.go')
  const face = read('photoly_server/internal/ai/people_model.go')
  const revision = model.match(/modelRevision\s*=\s*"([a-f0-9]+)"/)[1]
  const name = model.match(/modelName\s*=\s*"([^"]+)"/)[1]
  const fingerprint = face.match(/faceFingerprint\s*=\s*"([^"]+)"/)[1]
  assert.ok(page.includes(`/models/${revision.slice(0, 12)}/${name}/LICENSE`))
  for (const file of ['LICENSE.YuNet', 'LICENSE.SFace'])
    assert.ok(page.includes(`/models/people/${fingerprint}/${file}`))
  const supplements = JSON.parse(read('photoly_frontend_vue/licenses/supplements.json'))
  for (const [key, record] of Object.entries(supplements))
    for (const file of record.files ?? []) {
      const body = read(`photoly_frontend_vue/licenses/${file.file}`)
      assert.equal(createHash('sha256').update(body).digest('hex'), file.sha256, key)
    }
})
