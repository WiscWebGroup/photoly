// Pages Function template: forwards /__PREFIX__/* to the demo backend (same-origin for the browser, so no CORS).
// The workflow replaces __BACKEND__ and __PREFIX__ and copies this to functions/__PREFIX__/[[path]].js.
export async function onRequest({ request, params }) {
  const url = new URL(request.url)
  const path = [].concat(params.path ?? []).join('/')
  const target = new URL(`__BACKEND__/__PREFIX__/${path}`)
  target.search = url.search
  return fetch(new Request(target, request), { redirect: 'manual' })
}
