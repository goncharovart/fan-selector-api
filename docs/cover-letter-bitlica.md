# Cover letter — Bitlica · Backend Vibe Engineer (Golang)

Hi Bitlica team,

I'm applying for the Backend Vibe Engineer (Golang) role.

I want to be straightforward about the fit, because the title says "Vibe"
and that's the part where I think I can actually help you.

**What I bring:**

- Two years of production backend work on Wentmarket
  ([wentmarket.ru](https://wentmarket.ru)), a B2B HVAC platform I built
  end-to-end with Claude Code. 17 000+ polynomial fan curves in Postgres,
  sub-100ms duty-point matching, real money flowing through Yookassa,
  real orders shipping through CDEK. Order pipeline, soft-delete, idempotent
  payments, headless Bitrix24, 1C sync — all production, all running.
- A working spec-first workflow with AI coding agents. PRD → architecture →
  per-story acceptance criteria → code → tests → deploy. Not "ask Cursor to
  autocomplete," but a real loop where the spec gates the implementation
  and the AI is the productivity multiplier through every stage. BMad-style.
- Six years of HVAC engineering before the web stack — useful when the
  domain is technical (which a lot of backend work is).

**What I don't bring (yet):**

- Production Go. My primary stack is TypeScript/Node. To close the gap I
  built a real Go service in this same spec-driven style:
  [github.com/goncharovart/fan-selector-api](https://github.com/goncharovart/fan-selector-api).
  Cloud Run, Cloud SQL, OTel traces, distroless image, GitHub Actions CI,
  spec docs in `/specs`. It extracts Wentmarket's fan-matching engine into
  a clean Go microservice — small project, but every layer of what your
  posting asks for is there to look at and pick apart.
- GKE specifically. I've used Cloud Run on managed Kubernetes; Compute
  Engine for self-hosted. The primitives are the same — pods, services,
  configmaps, secrets — I just haven't operated GKE in production.

**Why this role:**

"Vibe Engineer" is the right name for what I already do. Most of the
"experienced senior" backend candidates you'll see have never shipped real
production with AI agents in the loop. I have. The Wentmarket repo is the
artifact; the fan-selector-api repo is the proof I can do it in your stack.

I'd value a 30-minute call to go through both repos with you and to learn
what Bitlica's "AI-native workflow" looks like in practice.

Open to relocation (Poland / Georgia / Belarus / other). Available immediately.

Thanks for reading,
Artur Goncharov
[@gonartur](https://t.me/gonartur) · +7 (995) 376-31-73
