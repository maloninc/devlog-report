# AGENTS.md
> This file is the canonical source. A Japanese translation may exist (e.g., `AGENTS.ja.md`) and may lag behind; when in doubt, prefer this file.

This repository follows an “AI-led, incremental development” workflow driven by small spec documents under `docs/`.
This file defines rules, required output formats, and quality gates for AI agents (Codex/ChatGPT, etc.).

---

## Scope
- This file is **only** instructions for AI agents (Codex/ChatGPT, etc.).

---

## 0. Goals
- Develop safely in **small units** based on spec documents in `docs/`
- Surface ambiguity early and present minimal decision points to the user
- Implement with minimal diffs while avoiding regressions

---

## 1. Assumptions / Workflow
- Specs are written in `docs/feat-*.md` / `docs/fix-*.md` (small and independent)
- The AI must **not** modify code until the user explicitly says **“Start implementation”**
- After implementation, the **user** runs acceptance tests; if OK, the **user** commits (the AI must not commit on its own)

---

## 2. AI Roles

### 2.1 Planning Phase (when the user asks to fill in the plan)
- Read the target spec document (e.g., `docs/feat-xxxx.md`) and the existing code
- If anything is unclear, list ambiguities/contradictions/missing details as “Open Questions / Issues”
- If the spec is sufficiently clear, append or update a checklist-style **`# Implementation Plan`** at the end of the doc
- The plan must include: minimal diff approach, regression checks, error behavior, response format, compatibility policy

### 2.2 Implementation Phase (only after “Start implementation”)
- Implement strictly following the checklist order
- Mark items as done when completed and add short notes if needed
- Perform regression checks (at minimum: provide manual/smoke-test steps)
- Do not do out-of-scope changes (e.g., refactors not required by the spec). If needed, propose a separate `docs/` spec.

### 2.3 Completion Phase (after finishing implementation)
- Provide acceptance steps for the user (curl examples / commands / expected outputs)
- Clearly document known constraints or non-goals
- If the user confirms success, suggest commit message(s) (but do not commit automatically)

---

## 3. Recommended Structure for Spec Docs (`docs/feat-*.md` / `docs/fix-*.md`)
Recommended sections:
- `# Overview` (1–3 lines)
- `# Background` (optional)
- `# Specification` (inputs, outputs, examples, errors, compatibility)
- `# Implementation Notes` (optional: reuse existing logic, “compatibility not required”, etc.)
- `# Implementation Plan` (filled/updated by the AI)

---

## 4. Required Format for `# Implementation Plan` (MUST FOLLOW)
The AI must write the plan in this exact checklist style. Keep items small enough to check off.

# Implementation Plan
* [ ] Do X
   - Detail a
   - Detail b
* [ ] Update route/handler/use-case
   - Parameters: xxx
   - Response: handling of md/json modes
   - Errors: e.g., when no data -> 404 + "not found"
* [ ] Regression checks
   - Confirm existing behavior for xxx is unchanged
   - Verify mode behavior (e.g., mode=json / default mode)
* [ ] Add acceptance steps
   - curl example(s)
   - Expected output example(s)

If the spec explicitly states compatibility is not required (e.g., unreleased), the plan must mention it.

---

## 5. Handling Ambiguity (Question Policy)
When creating the plan, if ambiguity is found:
- Ask only what is necessary to implement correctly (minimize questions)
- Provide choices (A/B/C) where possible
- Do not start implementation until ambiguity is resolved
- If the spec explicitly defines defaults (e.g., default to md) or policies (“compatibility not required”), follow them

---

## 6. Output / Errors / Compatibility (Quality Gates)
Both the plan and implementation must address at least the following:

### 6.1 For HTTP / API work
- Parameters: required/optional, types, defaults
- Modes (e.g., md/json) and Content-Type
- Error behavior (e.g., no data -> 404 + "not found" as `text/plain`, etc.)
- Ordering and rounding rules (e.g., seconds->minutes rounding up, descending sort, tie-breaking)

### 6.2 Compatibility Policy
- Explicitly state whether backward compatibility must be preserved
- If breaking changes are allowed, list removals (e.g., removing an endpoint) as checklist items

### 6.3 Regression Checks
- Always include checks to confirm existing “normal paths” are unchanged
  - Example: `/stats` md/json output without `project` remains unchanged

---

## 7. Implementation Rules (Allowed / Not Allowed)

### Allowed
- Minimal changes that satisfy the spec
- Reuse existing logic where possible (e.g., existing aggregation/classification)
- Add acceptance commands/examples (e.g., curl)

### Not Allowed
- Behavior changes not described in the spec (implicit spec creep)
- Large refactors (propose a separate `docs/` item instead)
- Starting implementation without an explicit “Start implementation” instruction
- Committing or releasing without explicit user approval

---

## 8. Completion Report Template (MUST PROVIDE)
After implementation, the AI must provide:
- Summary of changes (bullets)
- Acceptance steps (commands, expected results, verification points)
- Impact area and regression check results (what was tested; if not tested, why)
- Proposed commit message(s)
