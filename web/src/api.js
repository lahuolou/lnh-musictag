// 统一 API 封装：自动附带 CSRF 头（后端 csrfGuard 校验 X-Requested-With），
// 401 时通知全局登出；请求体由后端 limitBody 限制。
export async function api(path, opts = {}) {
  const headers = Object.assign(
    { 'Content-Type': 'application/json', 'X-Requested-With': 'LNH-MusicTag' },
    opts.headers || {}
  );
  const r = await fetch(path, Object.assign({ headers }, opts));
  if (r.status === 401 && !/\/login|\/me$/.test(path)) {
    window.dispatchEvent(new CustomEvent('unauth'));
    throw new Error('未登录或会话过期');
  }
  const data = await r.json().catch(() => null);
  if (!r.ok) throw new Error((data && data.error) || ('HTTP ' + r.status));
  return data;
}
