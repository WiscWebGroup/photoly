# ArtPlayer 5.4.0 notices

The published npm archive omits the full ArtPlayer license. Its distribution also
embeds option-validator (including kind-of), screenfull, and Hint.css. The build
collects installed dependency licenses and includes these reviewed upstream copies:

- `LICENSE`: https://github.com/zhw2590582/ArtPlayer/blob/5.4.0/LICENSE
- `screenfull-license`: https://github.com/sindresorhus/screenfull/blob/v6.0.2/license
  (MIT notice for the screenfull implementation vendored in ArtPlayer's
  `packages/artplayer/src/libs/screenfull.js`; ArtPlayer does not specify its version).
- `hint-css-LICENSE`: https://github.com/chinchang/hint.css/blob/v2.7.0/LICENSE
  (the vendored stylesheet identifies itself as Hint.css 2.7.0).

Review these sources and bundled code when upgrading ArtPlayer. The generator
rejects an unreviewed ArtPlayer version and missing or incomplete license files.
It runs without network access and writes `public/player-licenses.txt`, which is
included in Vite and Docker builds. These notices do not change Photoly's license.
