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

jsdom does not verify visual layout. Also check the rendered editor and sidebar
in light/dark mode, at desktop and narrow mobile widths, before release.
