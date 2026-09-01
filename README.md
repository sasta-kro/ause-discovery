# AUSE Discovery

AUSE Discovery is a searchable institutional archive of historical senior Projects and associated Artifacts.

## Foundation tooling

The repository uses Node.js `24.20.0` and pnpm `11.25.0`. The current local TypeScript static-analysis gate is `tsc -b`. `typescript-eslint` `8.69.0` does not support the required TypeScript `7.0.2`, so ESLint is limited to compatible JavaScript configuration until the fixed version baseline changes.

Use `make doctor` to inspect local tooling and available Docker fallback.
