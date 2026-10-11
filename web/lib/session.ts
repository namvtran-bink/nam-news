// Signed session cookie: "<expiry ms>.<hex HMAC-SHA256>". Uses Web Crypto so it
// runs in both middleware (edge) and route handlers (node).

export const SESSION_COOKIE = "quiet_session";
export const SESSION_MAX_AGE = 60 * 60 * 24 * 30; // 30 days, in seconds

const encoder = new TextEncoder();

function secret(): string {
  const s = process.env.APP_TOKEN;
  if (!s) throw new Error("APP_TOKEN is not set");
  return s;
}

async function hmac(data: string): Promise<string> {
  const key = await crypto.subtle.importKey(
    "raw",
    encoder.encode("quiet-session:" + secret()),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const sig = await crypto.subtle.sign("HMAC", key, encoder.encode(data));
  return Array.from(new Uint8Array(sig), (b) => b.toString(16).padStart(2, "0")).join("");
}

export async function createSession(): Promise<string> {
  const expires = String(Date.now() + SESSION_MAX_AGE * 1000);
  return `${expires}.${await hmac(expires)}`;
}

export async function verifySession(value: string | undefined): Promise<boolean> {
  if (!value) return false;
  const [expires, sig] = value.split(".");
  if (!expires || !sig || Number(expires) < Date.now()) return false;
  return safeEqual(sig, await hmac(expires));
}

export function safeEqual(a: string, b: string): boolean {
  const x = encoder.encode(a);
  const y = encoder.encode(b);
  let diff = x.length ^ y.length;
  for (let i = 0; i < Math.max(x.length, y.length); i++) diff |= (x[i] ?? 0) ^ (y[i] ?? 0);
  return diff === 0;
}
