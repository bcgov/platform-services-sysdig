# BCGov OpenShift Quota Review Handoff

Use this repo to review Registry quota edit requests while Billy is away.

The review is not just a metrics check. Read the full Registry request first,
then use cluster evidence to approve only the portion that is supported.

## 1. Read the Registry Request

Open the quota edit request in the Platform Services Registry and check:

- `SUMMARY`: changed fields and the full `Quota Contact & Justification`
- `ORIGINAL`: previous quota values when the summary is unclear
- `USER REQUEST`: requested values and team members
- `ADMIN DECISION` or comments: prior reviewer context, if present

Record:

- license plate and `OpenShift Cluster`
- quota contact name and email
- product owner and technical leads
- changed fields, old values, requested values
- affected environments
- full justification
- whether this is existing workload growth, planned new workload, or mixed

Do not make a recommendation before reading the full justification.

Namespace mapping:

```text
Development / Dev -> <license-plate>-dev
Test              -> <license-plate>-test
Production / Prod -> <license-plate>-prod
Tools             -> <license-plate>-tools
```

## 2. Run The Usage Check

Install requirements:

```bash
oc
curl
jq
awk
```

From this repo, run the script with the Registry cluster, license plate, and
affected environments:

```bash
./quota_review_queries <silver|gold|golddr|emerald> <license-plate> <env> [env...]
```

Examples:

```bash
./quota_review_queries silver 101ed4 dev test
./quota_review_queries gold abc123 tools prod
```

The script checks whether `oc` is logged in to the requested cluster. If not,
it runs browser login with `oc login -w`.

It prints a usage report for each namespace, such as `101ed4-dev` and
`101ed4-test`, using a 30-day window by default.

Use a longer window only when a long-lived workload may have infrequent peaks:

```bash
DAYS=100 ./quota_review_queries silver 101ed4 prod
```

If a query returns `NaN`, empty output, or a PromQL error, do not treat it as
zero usage. Mark that metric as missing evidence or run a corrected read-only
query.

## 3. Decide

Use one of:

- `Approve`: justification and evidence support the requested values
- `Partial Approve`: only some requested changes are supported
- `Do Not Approve Yet`: need may be credible, but the requested values cannot
  be validated

For existing workload growth, validate with actual use, peaks, restarts,
OOMKilled evidence, pod sizing, scaling behavior, and PVC growth.

For planned new workloads, ask for a resource bill of materials:

- components and purpose
- CPU and memory requests and limits
- replica counts, HPA, and rollout surge
- PVC count and size
- retention, ingestion, growth, and backup/object storage plan
- timeline
- temporary versus steady-state capacity

Do not reject a new workload only because current usage is low. Hold or stage
the numeric approval when component sizing is missing.

Storage review needs two checks:

- allocation: PVC requested capacity versus ResourceQuota
- filesystem usage: actual used bytes, utilization, and growth

If allocation is full but filesystem usage is low, approve only enough to
unblock the next deployment plus modest headroom unless the team explains why
the full value is needed now.

## 4. Write The Review

Use this format:

```text
Decision: Approve / Partial Approve / Do Not Approve Yet

Request and justification:
- <requested changes>
- <brief summary of the submitted justification>

Reason:
- <justification assessment>
- <usage or sizing evidence>

Approved changes:
- <change or "None">

Held changes:
- <change or "None">

Missing evidence:
- <item or "None">
```

Team fields, when needed:

```text
Quota contact: <name and email>
Product Owner: <name and email>
Primary Technical Lead: <name and email>
Secondary Technical Lead: <name and email or None>
Additional members: <members or None>
```

## 5. Email Templates

Templates are in `templates/`:

- `partial-approve.txt`
- `clarification-needed.txt`
- `storage-staged.txt`
- `missing-justification.txt`

Address the quota contact. Copy the technical lead and product owner when it is
useful.
