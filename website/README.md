# px1 website

A single static page (`index.html`) plus the install script it serves at `/install.sh`. No build step, no dependencies.

## Deploy to Vercel

1. Push this repo to GitHub (`programmerpranit/px1`).
2. In the Vercel dashboard: **New Project** → import the repo.
3. Set **Root Directory** to `website`.
4. Framework Preset: **Other** (no build command, no output directory — it's already static).
5. Deploy. Attach your domain (`px1.pranitpatil.com`) under Project → Settings → Domains.

That's it — `curl -fsSL https://px1.pranitpatil.com/install.sh | sh` will work once the domain is attached, since `install.sh` here matches the one at the repo root.

## Local preview

```bash
cd website && python3 -m http.server 8000
```

## Keeping install.sh in sync

`website/install.sh` is a copy of the repo-root `install.sh`, customized to point at this fork (`programmerpranit/px1`, `PX1_*` env vars). If you change the root `install.sh`, copy the same change here — this is the file that actually gets served to users running the curl one-liner.
