# Workload Stability & Scaling Dashboard (Per Namespace)

This dashboard is a “health + scalability” view of your namespace. It’s designed for quick triage: when everything looks quiet, your workloads are generally stable; when a line spikes, it points to a specific class of problem (performance, availability, rollout health, scaling ceiling, network reliability, or maintenance readiness).

## What “good” looks like

It follows a simple rule: **0 is good** — most panels represent “bad events” or “unhealthy states,” so when the values stay at 0, your workloads are generally stable and scaling normally. When a line spikes above 0, it’s a strong clue that something changed (performance, availability, rollout health, scaling ceiling, network reliability, or maintenance readiness).

---

## How to use it (recommended workflow)

1. **Set the time range first**
   Pick a window that includes the symptoms (last 1h, 6h, 24h, 7d). Shorter windows help you see causality; longer windows help you spot recurring patterns.

2. **Start with “does anything look abnormal?”**
   If most panels are flat at zero, your namespace is likely stable.

3. **When something spikes, follow the panel’s “what it means” and “what to do next”**
   Each panel is intentionally scoped to a symptom category and maps to common causes and next steps.

4. **Correlate panels**
   Many failures show up in multiple places:
   - CPU throttling spikes + scaling pressure often means the app is hitting CPU limits or scaling ceiling.
   - Deployment gap + stuck states often means rollout is blocked (pull errors, crash loops, readiness issues).
   - OOM-related signals + restarts typically means memory pressure or requests/limits mis-sized.

---

## What each panel tells you (and how it helps)

### CPU throttling pressure

What it shows:

- Whether containers are being delayed because they’re hitting CPU limits.

How it helps:

- Explains “my app is slow but not down”.
- Identifies when CPU limits are too tight for bursty workloads, or when the workload needs to scale out.

Typical next steps:

- Check if CPU limits are set too low for peak demand.
- Consider increasing replicas, tuning autoscaling, or revisiting request/limit strategy for latency-sensitive paths.

---

### Crash & restart trends

What it shows:

- Whether containers are restarting frequently over time.

How it helps:

- Surfaces instability: crashes, config regressions, dependency failures, bad deploys.
- Helps you correlate restarts with deployments, traffic spikes, or config changes.

Typical next steps:

- Look at pod logs around restart times.
- Check recent changes (image tag, config, secrets, dependency endpoints).
- Review readiness/liveness probes and resource sizing.

---

### OOM kill / abnormal termination signals

What it shows:

- Evidence of memory-related terminations or hard failures.

How it helps:

- Distinguishes “normal restarts” from “resource exhaustion”.
- Flags memory pressure as the root cause of instability.

Typical next steps:

- Review memory usage patterns and memory requests/limits.
- Confirm whether the workload has realistic memory sizing for peak load.
- Investigate memory leaks or unbounded in-memory caching.

---

### Autoscaling ceiling pressure (HPA headroom)

What it shows:

- Whether your autoscaler is getting close to its configured maximum capacity.

How it helps:

- Explains why performance degrades even though autoscaling is enabled.
- Highlights “we _want_ to scale more, but cannot”.

Typical next steps:

- Increase max replica ceiling (if appropriate) and ensure the cluster has capacity.
- Validate that scaling signals are correct (CPU, memory, or custom metrics).
- Ensure requests/limits are set so new replicas can schedule successfully.

---

### Network error rate

What it shows:

- Indicators of network-level errors for workloads in the namespace.

How it helps:

- Helps separate “app bug” vs “network/path issue”.
- Useful when you see timeouts, connection resets, or intermittent failures.

Typical next steps:

- Confirm upstream/downstream dependencies are healthy.
- Check service endpoints, DNS, egress policies, and certificate/MTLS behavior (if applicable).
- Correlate with deployment events or sudden traffic shifts.

---

### Deployment gap (desired vs available)

What it shows:

- When your desired replicas are not actually available to serve traffic.

How it helps:

- Quickly answers “is my rollout healthy?” and “am I under-provisioned right now?”
- Detects stuck or degraded rollouts even when some pods are running.

Typical next steps:

- Inspect rollout status and events.
- Look for readiness failures, image pull problems, crash loops, or insufficient capacity.

---

### Stuck states (pods not progressing)

What it shows:

- Pods in waiting/problem states that prevent workloads from becoming ready.

How it helps:

- Pinpoints rollout blockers like image pull failures or crash loops.
- Provides fast classification of “why isn’t it coming up?”

Typical next steps:

- Check image registry access, image tag validity, pull secrets.
- Review container logs and startup configs.
- Validate required dependencies (DB, secrets, config maps) exist and are accessible.

---

### Overall issue signal (combined “something is wrong” indicator)

What it shows:

- A summarized signal that flips non-zero when the namespace has one or more major health problems (e.g., pods not ready and/or stuck).

How it helps:

- Provides a quick “red/yellow/green” style check without scanning every panel.
- Useful for on-call triage: “where should I focus first?”

Typical next steps:

- Use it as an entry point, then drill into the specific panels to find the failure class.

---

### Node drain blocker (maintenance readiness)

What it shows:

- Whether disruption policies could block node drains / maintenance operations.

How it helps:

- Prevents surprises during cluster maintenance.
- Flags namespaces where voluntary disruptions may stall maintenance until availability constraints are met.

Typical next steps:

- Confirm disruption budget settings align with availability goals.
- Ensure replica counts and readiness allow voluntary evictions.
- Coordinate changes before planned maintenance windows.

---

## Common patterns and what they usually mean

- **CPU throttling + autoscaling pressure**
  Your workload is CPU constrained and cannot scale further. Expect latency and timeouts under load.

- **Deployment gap + stuck states**
  Rollout is not completing (image pull issues, crash loops, readiness failures, or capacity/scheduling constraints).

- **Restarts + OOM signals**
  Memory sizing is likely insufficient or workload behavior has changed (traffic, data shape, caching, leaks).

- **Network error spikes without restarts**
  Often dependency/network path issues (upstream health, DNS, egress, certificates) rather than core app crashes.

---

## Tips for app teams

- Use this dashboard during PR/Release validation: confirm no new instability patterns appear post-deploy.
