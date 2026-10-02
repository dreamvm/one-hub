# UI regression tests

Run from `web/` with Node 22.20 and Yarn 1.22.22:

```sh
yarn install --frozen-lockfile --non-interactive
yarn test
yarn lint
yarn build
```

The tests render the real channel/token editors, Formik validation, MUI controls,
date picker, model selector, sidebar navigation, theme and translations in jsdom. API reads/writes and the
external Monaco/icon renderers are mocked: no production accounts, keys or
paid model calls are used. Test fixtures must remain synthetic.

Coverage includes loading/submit guards, read failure and retry, malformed
configuration, stale responses, dirty-edit preservation, session reopening,
group restrictions, new channels, independent plugin expansion and saved
values, and mini-sidebar navigation/Escape dismissal.

Additional coverage includes token read failures/mismatched IDs, blocked submits,
retry, stale reads and saves, reopen/create isolation, admin read/update routing,
save-error recovery, latest-only channel search/pagination, end-date field and
calendar-button selection, start-then-end selection, invalid dates, and one-time
model-icon fallback in light/dark themes. Channel list tests isolate unrelated
row/editor components while retaining the real toolbar, pagination and effects.

The existing compatibility workflow runs these tests, lint and build for PRs
and the exact source revision requested by the manual image workflow. This
does not enable automatic image publication or deployment.

`yarn test` first runs `yarn test:dev`: Node's native test runner loads the
top-level Vite used by dev/build, rather than Vitest's separate Vite dependency.
The real project plugins run against a temporary synthetic root with dotenv
loading and browser opening disabled, on loopback and an OS-assigned port.
The 30 leaf checks cover HTTP deny/query and absolute-path forms, optimizer map
traversal and encoded forms, WebSocket module access, and legitimate raw/inline,
in-cache map, HMR and server-side module controls. Fixtures are removed after
the server and socket close. No real credentials or upstream services are used.

These checks exercise the Unix dev-server boundary; Windows ADS/short filenames
and UNC editor behavior still require native verification. Upgrading Vite does
not establish that all frontend runtime or tooling advisories are resolved.

`yarn test:deps` loads the explicit top-level Axios browser distribution. Each
synthetic prototype precondition runs in its own child process; normal JSON,
explicit DELETE data/reviver, query encoding, Blob and cancellation stay valid.
A second fixture uses actual jsdom XMLHttpRequest and two synthetic loopback
HTTP servers on random ports, including an allowed CORS price request. The 20
leaf checks are followed by four Vitest checks through the real application API
factories, with only responses, notifications and store simulated. These verify
library and API compatibility; they do not prove an application pollution
source, native browser behavior or production end-to-end acceptance.

jsdom does not verify visual layout. Also check the rendered editor and sidebar
in light/dark mode, at desktop and narrow mobile widths, before release.

The dependency suite also runs 16 PostCSS leaf checks against the version used
by top-level Vite: standalone map boundaries, real Vite build/import processing,
and legitimate CSS, CSS Modules, SCSS, local assets and source maps. Only
synthetic temporary files are observed. These checks cover PostCSS processing;
Vite's separate development source-map loader remains a documented boundary in
`docs/POSTCSS_FILE_BOUNDARY.md`, not a claimed fix of every source-map read.

The Rollup-only suite was retired when the Vite 8 candidate removed Rollup
from the dependency graph. Its regression and acceptance evidence remain in
PR #68 history and docs/ROLLUP_OUTPUT_BOUNDARY.md. Vite source-map tests and
application tests remain active; this is not an independent-review result.
