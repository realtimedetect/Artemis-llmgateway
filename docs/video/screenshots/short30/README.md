# Screenshot Inputs for 30s Video

Place PNG screenshots in this folder using these exact names:

- 01-overview.png
  Use the dashboard overview page showing usage cards and provider health.

- 02-providers.png
  Use the providers page showing configured providers and health state.

- 03-routes.png
  Use the routes page showing slug, provider, model, and policy fields.

- 04-costs.png
  Use the cost settings page showing spend, groups, or pricing rules.

- 05-audits.png
  Use the audit logs page with one expanded row if possible.

Recommended capture settings:
- PNG format
- 16:9 aspect ratio if possible
- Browser zoom at 110%
- Avoid cropped sidebars or devtools

After adding screenshots, rerun:

powershell -ExecutionPolicy Bypass -File "C:\Users\vinod\OneDrive\llm_gateway\docs\video\generate_short_30s_video.ps1"
