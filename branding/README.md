# LocasOS Visual Identity

**Approved wordmark:** LOCASOS  
**Approved tagline:** YOUR LOCAL AI APPLIANCE OS

The approved artwork uses a silver metallic L over a mountain/lake/aurora circle, blue illuminated rim, dark navy background, and silver/blue LOCASOS lettering. Do not regenerate or recolor it for releases without explicit design approval.

## Committed integration

- `branding/locasos-icon-96.webp`: small cropped rendition of the approved emblem, committed for repository reference.
- `cmd/locas-core/brand-icon.webp`: same image embedded in the actual local Go dashboard binary; `GET /brand/icon.webp`, header and favicon.
- `web/public/brand-icon.webp`: same image for the separate development web UI.

## Full-resolution production assets

The complete brand kit, generated from the approved image, is provided as `locasos-brand-assets.zip` in the project conversation. **The high-resolution assets are not yet checked into this repository**. Copy them into `branding/assets/` and commit as a separate binary-assets PR; do not use a regenerated or unapproved source image.

Expected files:
- `locasos-logo-full.png` — canonical approved artwork and exact tagline
- `locasos-dashboard-logo.webp` — full site hero / welcome artwork
- `locasos-icon-512.png`, `locasos-icon-256.png`, `locasos-icon-192.png`, `locasos-icon.ico`, `locasos-favicon.png` — icons
- `locasos-boot-1920x1080.png`, `locasos-boot-1280x720.png` — boot background artwork

The small embedded dashboard icon is suitable for a header/favicon but not a full-resolution splash. For full fidelity, wire the high-resolution images after their bytes are committed.

## Native ISO and boot screen

**LocasOS does not currently build a bootable bare-metal ISO.** A bootable ISO cannot be branded just by renaming its file or setting a Windows `.ico`. Future native image work should:
1. Install `locasos-boot-1920x1080.png` as a GRUB background and use a matching Plymouth theme.
2. Use the emblem for installer UI and installed desktop/application icons where applicable.
3. Configure the ISO volume label `LOCASOS` and appropriate boot menu metadata.
4. Test UEFI boot in a disposable VM, including no-graphics fallback and low-resolution consoles.
5. Keep the splash purely cosmetic; never block boot if artwork is missing.

Do not advertise ISO branding as complete until a real bootable ISO is built and tested.
