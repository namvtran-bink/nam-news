export default async function Login({ searchParams }: { searchParams: Promise<{ error?: string }> }) {
  const { error } = await searchParams;
  return (
    <main className="page first-run">
      <h1 className="title">Quiet</h1>
      <form className="login" method="post" action="/api/login">
        <label htmlFor="password">Password</label>
        <input id="password" name="password" type="password" autoComplete="current-password" required autoFocus />
        {error && (
          <p className="error" role="alert">
            That password didn&apos;t work.
          </p>
        )}
        <button type="submit" className="primary">
          Open today&apos;s page
        </button>
      </form>
    </main>
  );
}
