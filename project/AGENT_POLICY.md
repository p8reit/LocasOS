# Agent execution policy

Agents operate per GitHub issue with limited file ownership, explicit acceptance criteria and branch naming `agent/<role>/LOS-NNN`.

Model routing: start with low-cost local coding models for narrow tests, scripts, docs, and UI components; escalate complex concurrency, architecture, and security review to stronger reasoning models. Record actual model, token use, attempts, and tests; never claim cost savings without measurement.

Required: issue-linked draft PR, changed-file summary, test commands/results, risk assessment, and explicit blockers. Human/integration approval before merge. No production deployments, no arbitrary host commands through the core/agent protocol, and no secrets in prompts or commits.

Agent roles: A1 Platform; A2 Backend; A3 Containers; A4 GPU; A5 Frontend; A6 Security/QA. Agents are assignment definitions, not running workers.
