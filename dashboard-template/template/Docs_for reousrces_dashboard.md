# Resource Utilization & Quota Dashboard (CPU / Memory / Storage)

This dashboard helps you understand, at a namespace level, the relationship between **what your workloads actually consume** and **what your namespace is allowed to request** under the platform ResourceQuota. It’s built for day-to-day troubleshooting and, more importantly, for supporting (or challenging) **quota increase requests** with evidence.

The key idea is that Kubernetes quotas are enforced on **requested resources** (what you declare), while runtime usage reflects **what you actually consume**. By putting those side-by-side, you can answer: “Are we truly constrained, or are we simply over-requesting?” Kubernetes ResourceQuota is explicitly a per-namespace mechanism to limit aggregate resource consumption/requests. :contentReference[oaicite:0]{index=0}

---

## What you’ll get from this dashboard

You can use it to:

- **Validate quota increase requests** by checking whether request ceilings were actually approached over the selected time range.
- **Right-size requests and limits** by comparing “actual usage” vs “requested/allowed”.
- **Identify the top containers** that are consuming the largest share of their CPU/memory limits (the usual source of hotspots).
- **Catch storage risk early** by seeing PVC utilization trends and which claims are closest to filling.

---

## How to interpret the charts (high level)

CPU and memory sections generally show three concepts:

- **Actual usage**: what your containers are consuming at runtime (resource usage from the container/cgroup view). :contentReference[oaicite:1]{index=1}
- **Request usage**: how much “requested” resource your namespace is currently consuming against quota (this is what quota accounting cares about).
- **Quota hard (limit)**: the maximum request budget your namespace is allowed before the platform will block additional scheduling/creation.

For quota approvals:

- If **request usage frequently approaches quota hard**, you have a strong case that you’re constrained by quota.
- If **actual usage is consistently far below request usage**, the better fix is usually request/limit tuning (and you may not need more quota).
- If **a small number of containers are near their limits**, focus on those first—raising quota may not fix the real bottleneck.

---

## Time range matters (use the bottom time selector)

This dashboard is designed to be read over a time window (incident window, peak traffic window, last 7 days, etc.). Sysdig supports time variables like `$__interval` and `$__range` so the panels automatically adapt to the time range you choose. :contentReference[oaicite:2]{index=2}

A common workflow for quota decisions:

- Look at **last 24h** to catch recent peaks.
- Look at **last 7d** to confirm it’s not a one-off.
- If you’re planning capacity, consider **last 30d** to see recurring patterns.

---

## Why “requests” and “usage” can diverge (and why that’s useful)

It’s normal for “requested” resources to be higher than actual usage—requests are a scheduling and budgeting signal, while usage is real consumption. When these diverge significantly, it usually indicates an opportunity to reduce requested resources without hurting performance, freeing capacity for everyone on the platform. Kubernetes treats requests/quotas as policy boundaries at the namespace level. :contentReference[oaicite:3]{index=3}

---

## Storage section

Storage panels help you see:

- Which PVCs are closest to full (risk of application issues when storage fills).
- Overall PVC usage trends vs your namespace’s storage request budget and limits.

Use this when requesting storage quota changes or when you see app errors that often correlate with full disks.

---

## Scope and consistency

This dashboard relies on the Sysdig dashboard scope (`$__scope`) so it works consistently across clusters/namespaces and avoids label mismatches that can happen when metrics come from different sources. Sysdig explicitly recommends using `$__scope` for PromQL panels so the dashboard scope applies reliably. :contentReference[oaicite:4]{index=4}
