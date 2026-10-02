# Frontend

The production UI is a static **Vue 3 + Vite** bundle. You normally serve it from any object store or generic web server container (NGINX, Caddy, `ingress-nginx` static backend, etc.) independent of the Go microservices.

## Runtime configuration

At build time (CI `docker build`, `npm run build`, or equivalent), set:

| Build argument / env | Purpose |
|----------------------|---------|
| **`VITE_CONTROLLER_BASE_URL`** | Forces the first candidate Connect base URL (`https://controller.example.com` or `http://internal-svc:8080`). Must include scheme, omit trailing slash. |

If you omit the variable, the SPA still works: it probes common candidates (most recent saved controller URL, same-host `:8080`, same-origin). Successful manual connections are kept in browser local storage (newest first, up to five) and can be picked again from the controller-not-reachable screen. For predictable behaviour in production, bake the correct controller URL into the build.

## Operator checklist

1. Run the controller behind TLS if exposed beyond the cluster mesh.
2. Ensure CORS rules on the controller allow your UI origin (the controller enables permissive CORS today—tighten in hardened environments).
3. Document the controller URL in your portal so operators can paste it into the UI when switching environments.
