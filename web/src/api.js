export async function api(path, opts = {}) {
  const r = await fetch(path, Object.assign({ headers: { 'Content-Type': 'application/json' } }, opts));
  if (r.status === 401 && !/\/login|\/me$/.test(path)) {
    window.dispatchEvent(new CustomEvent('unauth'));
    throw new Error('未登录或会话过期');
  }
  const data = await r.json().catch(() => null);
  if (!r.ok) throw new Error((data && data.error) || ('HTTP ' + r.status));
  return data;
}
