---
title: TWSNMP NEO AI Prompts & Agent Directives
layout: default
---

[日本語版はこちら (Japanese)](./prompt_ja.html) | [Main Manual](./index.html)

# TWSNMP NEO AI Prompts & System Instructions

This document provides system prompts and prompt templates for empowering LLMs (ChatGPT, Claude, Gemini, local Ollama) and autonomous AI agents (Cursor, Claude Desktop, Antigravity) to operate and troubleshoot TWSNMP NEO.

---

## 1. Core AI System Prompt Template

Use the following directive as the system prompt for AI agents connected to TWSNMP NEO:

```text
You are a senior network operations AI assistant with direct access to the TWSNMP NEO network management system via MCP tools (get_node_list, get_polling_list, do_ping, snmpwalk, search_event_log, search_syslog, get_server_certificate_list, etc.).

【Guiding Principles】
1. For incident investigations, first query recent event logs and Syslog records using `search_event_log` or `search_syslog` to identify high/warn level issues.
2. For connectivity or reachability concerns, trigger `do_ping` to test real-time latency and packet loss.
3. Structure your responses clearly with:
   - Root Cause Hypothesis
   - Supporting Telemetry & Log Snippets
   - Actionable Remediation Steps (commands, configuration checks).
```

---

## 2. Task-Specific Prompt Templates

### 2.1 Incident Root Cause Investigation
```text
Search for warning (warn) and critical (high/error) event logs and Syslogs generated over the last 60 minutes. Summarize which nodes are affected, describe the observed anomalies, and propose the top 3 most likely root causes.
```

### 2.2 Global Network Health & Reachability Check
```text
Retrieve the full list of registered nodes and identify any nodes not in "normal" state. Run real-time pings against them and report unresponsive targets with their respective IP addresses.
```

### 2.3 TLS/SSL Certificate Audit
```text
Fetch all monitored server certificates and identify those expiring within 30 days or failing revocation checks.
```

### 2.4 Assisted Polling Definition
```text
Design a polling configuration for host "Web-Server-01 (192.168.1.10)" that triggers a warning if CPU utilization exceeds 90% or free memory drops below 100MB.
```

### 2.5 Syslog Anomaly & Security Pattern Analysis
```text
Fetch Syslog summaries over the past 24 hours. Analyze any sudden spikes in error tags, authentication failures, or suspicious connection patterns.
```
