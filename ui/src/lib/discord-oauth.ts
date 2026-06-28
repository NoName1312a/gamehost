import { invoke } from '@tauri-apps/api/core'
import { listen } from '@tauri-apps/api/event'
import { open } from '@tauri-apps/plugin-shell'
import { supabase } from './supabase'

/** Sign in with Discord via a loopback PKCE flow: start a local server,
 *  open the provider URL in the browser, capture the code, exchange it. */
export async function signInWithDiscord(): Promise<void> {
  const port = await invoke<number>('start_oauth_loopback')
  const redirectTo = `http://localhost:${port}/`

  const codePromise = new Promise<string>((resolve, reject) => {
    let unlisten: (() => void) | undefined
    const timer = setTimeout(() => {
      unlisten?.()
      reject(new Error('Sign-in timed out. Please try again.'))
    }, 120_000)
    listen<string>('oauth-code', (e) => {
      clearTimeout(timer)
      unlisten?.()
      resolve(e.payload)
    }).then((u) => { unlisten = u })
  })

  const { data, error } = await supabase.auth.signInWithOAuth({
    provider: 'discord',
    options: { redirectTo, skipBrowserRedirect: true },
  })
  if (error) throw new Error(error.message)
  if (data.url) await open(data.url)

  const code = await codePromise
  const { error: exErr } = await supabase.auth.exchangeCodeForSession(code)
  if (exErr) throw new Error(exErr.message)
}
