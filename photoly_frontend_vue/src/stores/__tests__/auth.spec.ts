import { describe, expect, it } from 'vitest'
import { http as mswHttp } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { useAuthStore } from '../auth'

describe('auth store', () => {
  it('reads the raw token stored in localStorage', () => {
    localStorage.setItem('HRD-Token', 'abc')
    const auth = useAuthStore()
    expect(auth.isLoggedIn).toBe(true)
    expect(auth.avatarUrl).toBe(`${API}/user/getAvatar/abc`)
  })

  it('picks up a token written behind its back on refresh()', () => {
    const auth = useAuthStore()
    expect(auth.isLoggedIn).toBe(false)
    localStorage.setItem('HRD-Token', 'late')
    auth.refresh()
    expect(auth.token).toBe('late')
  })

  it('setToken stores the raw string; clear removes it', () => {
    const auth = useAuthStore()
    auth.setToken('t1')
    expect(localStorage.getItem('HRD-Token')).toBe('t1')
    auth.clear()
    expect(localStorage.getItem('HRD-Token')).toBeNull()
    expect(auth.isLoggedIn).toBe(false)
  })

  it('signOut calls the API with the token, and forgets it even if the call fails', async () => {
    const calls = recordRequests()
    server.use(mswHttp.post(`${API}/user/signOut/`, () => ok()))
    const auth = useAuthStore()
    auth.setToken('t2')
    await auth.signOut()
    expect(calls).toMatchObject([{ method: 'POST', path: '/user/signOut/' }])
    expect(auth.isLoggedIn).toBe(false)

    auth.setToken('t3')
    server.use(mswHttp.post(`${API}/user/signOut/`, () => Response.error()))
    await expect(auth.signOut()).rejects.toThrow()
    expect(auth.isLoggedIn).toBe(false)
  })
})
