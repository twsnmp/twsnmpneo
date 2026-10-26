<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _, locale } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    Bot,
    ShieldAlert,
    ShieldCheck,
    FileSpreadsheet,
    Network,
    Cpu,
    HardDrive,
    Activity,
    Plus,
    Zap,
    CheckCircle2,
    AlertTriangle,
    ChevronDown,
    ChevronRight,
    Sparkles,
    Copy,
    Check,
    SlidersHorizontal,
    Server,
  } from "@lucide/svelte";
  import NodeDialog from "../../components/NodeDialog.svelte";
  import PollingDialog from "../../components/PollingDialog.svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import ReportAIExplainModal from "../../components/ReportAIExplainModal.svelte";
  import { formatTimeStr, renderBytes } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import {
    fetchLogReport,
    resetLogReport,
    saveNode,
    savePolling,
    type TwAgentInfoEnt,
    type TwAgentAlertEnt,
    type TwAgentFIMEnt,
    type TwAgentSCAEnt,
    type TwAgentTCPEnt,
    type NodeEnt,
    type PollingEnt,
  } from "../../api";
  import { showConfirm, showAlert } from "../../stores/modalStore";

  let {
    searchQuery = "",
    nodes = [],
    pollings = [],
    onRefresh = () => {},
    loading = false,
  }: {
    searchQuery?: string;
    nodes?: NodeEnt[];
    pollings?: PollingEnt[];
    onRefresh?: () => void;
    loading?: boolean;
  } = $props();

  type TabType = "agents" | "alerts" | "fim" | "sca" | "tcp";
  type AlertFilterType = "all" | "threat" | "compliance" | "benign";

  let activeTab = $state<TabType>("agents");
  let alertFilter = $state<AlertFilterType>("all");

  let agents = $state<TwAgentInfoEnt[]>([]);
  let alerts = $state<TwAgentAlertEnt[]>([]);
  let fimEvents = $state<TwAgentFIMEnt[]>([]);
  let scaFindings = $state<TwAgentSCAEnt[]>([]);
  let tcpConns = $state<TwAgentTCPEnt[]>([]);

  let internalLoading = $state(false);
  let actionBusyId = $state<string | null>(null);
  let expandedId = $state<string | null>(null);
  let copiedHashId = $state<string | null>(null);
  let showAiExplainModal = $state(false);

  // Dialog states for custom node / polling creation
  let showNodeModal = $state(false);
  let nodeToEdit = $state<NodeEnt | null>(null);
  let showPollingModal = $state(false);
  let pollingToEdit = $state<PollingEnt | null>(null);

  // Chart DOM refs & instances
  let resourceChartElem = $state<HTMLDivElement | null>(null);
  let resourceChartInstance: echarts.ECharts | null = null;
  let securityChartElem = $state<HTMLDivElement | null>(null);
  let securityChartInstance: echarts.ECharts | null = null;
  let detailChartElem = $state<HTMLDivElement | null>(null);
  let detailChartInstance: echarts.ECharts | null = null;

  const isJaLocale = $derived($locale?.startsWith("ja") ?? true);

  let sortColumn = $state("LastTime");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  const normMAC = (m?: string) => (m || "").replace(/-/g, ":").toUpperCase().trim();

  const findMatchedNode = (agent: TwAgentInfoEnt): NodeEnt | undefined => {
    const aIP = (agent.IP || "").trim();
    const aMAC = normMAC(agent.MAC);
    const aHost = (agent.Hostname || "").toLowerCase().trim();
    const aID = (agent.AgentID || "").toLowerCase().trim();

    for (const n of nodes) {
      const nIP = (n.ip || n.IP || "").trim();
      const nMAC = normMAC(n.mac || n.MAC);
      const nName = (n.name || n.Name || "").toLowerCase().trim();
      if (aIP && nIP && aIP === nIP) return n;
      if (aMAC && nMAC && aMAC === nMAC) return n;
      if (aHost && nName && aHost === nName) return n;
      if (aID && nName && aID === nName) return n;
    }
    return undefined;
  };

  const getAgentPollings = (agent: TwAgentInfoEnt, matchedNode?: NodeEnt): PollingEnt[] => {
    if (!matchedNode) return [];
    const nid = matchedNode.id || matchedNode.ID || "";
    if (!nid) return [];
    return pollings.filter((p) => {
      const pNodeId = p.node_id || p.NodeID || "";
      if (pNodeId !== nid) return false;
      const mode = (p.mode || p.Mode || "").toLowerCase();
      const filter = (p.filter || p.Filter || "").toLowerCase();
      const name = (p.name || p.Name || "").toLowerCase();
      return (
        mode === "twagent" ||
        filter.includes("twsnmpneo/agent") ||
        filter.includes("twagentneo") ||
        name.includes("twagent")
      );
    });
  };

  const osToIcon = (osStr?: string): string => {
    const s = (osStr || "").toLowerCase();
    if (s.includes("win")) return "windows";
    if (s.includes("darwin") || s.includes("mac")) return "laptop";
    return "server";
  };

  const loadAll = async () => {
    internalLoading = true;
    try {
      const [infoList, alertList, fimList, scaList, tcpList] = await Promise.all([
        fetchLogReport<TwAgentInfoEnt>("twAgentInfo"),
        fetchLogReport<TwAgentAlertEnt>("twAgentAlert"),
        fetchLogReport<TwAgentFIMEnt>("twAgentFIM"),
        fetchLogReport<TwAgentSCAEnt>("twAgentSCA"),
        fetchLogReport<TwAgentTCPEnt>("twAgentTCP"),
      ]);
      agents = infoList;
      alerts = alertList;
      fimEvents = fimList;
      scaFindings = scaList;
      tcpConns = tcpList;
    } catch (err) {
      console.error("Failed to load twagentneo reports:", err);
    } finally {
      internalLoading = false;
    }
  };

  export const refresh = () => {
    void loadAll();
  };

  export async function handleClear(): Promise<void> {
    const tabKindMap: Record<TabType, { kind: string; name: string }> = {
      agents: { kind: "twAgentInfo", name: $_("report.twagentTabAgents") || "エージェント一覧" },
      alerts: { kind: "twAgentAlert", name: $_("report.twagentTabAlerts") || "AIセキュリティアラート" },
      fim: { kind: "twAgentFIM", name: $_("report.twagentTabFim") || "ファイル整合性監視 (FIM)" },
      sca: { kind: "twAgentSCA", name: $_("report.twagentTabSca") || "構成監査 (SCA)" },
      tcp: { kind: "twAgentTCP", name: $_("report.twagentTabTcp") || "プロセスTCP通信" },
    };
    const target = tabKindMap[activeTab];
    const ok = await showConfirm({
      title: $_("common.confirmClear") || "データ消去の確認",
      message: $_("report.confirmClearItem", { values: { name: target.name } }) || `${target.name}データを全消去しますか？`,
      type: "warning",
      confirmText: $_("common.clear") || "消去",
    });
    if (!ok) return;
    try {
      await resetLogReport(target.kind);
      await loadAll();
      onRefresh();
    } catch (err: any) {
      showAlert({
        title: $_("common.error") || "エラー",
        message: $_("report.alertResetFailed", { values: { error: err.message || err } }),
        type: "danger",
      });
    }
  }

  // One-click Node Creation
  const createNodeForAgent = async (agent: TwAgentInfoEnt, idxOffset = 0): Promise<NodeEnt> => {
    const existing = findMatchedNode(agent);
    if (existing) return existing;
    const count = nodes.length + idxOffset;
    const x = 140 + (count % 8) * 110;
    const y = 140 + Math.floor(count / 8) * 110;
    const nodeName = agent.Hostname || agent.AgentID || agent.IP || "twagentneo";
    const saved = await saveNode({
      id: "",
      name: nodeName,
      ip: agent.IP || "127.0.0.1",
      mac: agent.MAC || "",
      icon: osToIcon(agent.OS),
      state: agent.Status === "online" ? "normal" : "warn",
      addr_mode: agent.IP ? "ip" : "host",
      descr: `twagentneo ${agent.Version || ""} (${agent.OS || "agent"} / ID: ${agent.AgentID || "-"})`.trim(),
      x,
      y,
    });
    return saved;
  };

  const handleQuickAddNode = async (agent: TwAgentInfoEnt, e?: Event) => {
    e?.stopPropagation();
    if (actionBusyId) return;
    actionBusyId = agent.ID;
    try {
      const saved = await createNodeForAgent(agent);
      onRefresh();
      showAlert({
        title: $_("report.twagentNodeAddedTitle") || "マップにノードを追加しました",
        message: $_("report.twagentNodeAddedMsg", {
          values: { name: saved.name || saved.Name, ip: saved.ip || saved.IP },
        }) || `ノード「${saved.name || saved.Name} (${saved.ip || saved.IP})」をマップに追加しました。`,
        type: "info",
      });
    } catch (err: any) {
      showAlert({
        title: $_("common.error") || "エラー",
        message: err.message || String(err),
        type: "danger",
      });
    } finally {
      actionBusyId = null;
    }
  };

  const handleBulkAddUnregisteredNodes = async () => {
    const unregistered = agents.filter((a) => !findMatchedNode(a));
    if (unregistered.length === 0) return;
    const ok = await showConfirm({
      title: $_("report.twagentBulkAddNodesTitle") || "未登録エージェントの一括ノード追加",
      message: $_("report.twagentBulkAddNodesConfirm", { values: { count: unregistered.length } }) ||
        `未登録の ${unregistered.length} 台のエージェントをマップに一括追加しますか？`,
      type: "info",
      confirmText: $_("report.btnAddNode") || "追加",
    });
    if (!ok) return;
    actionBusyId = "bulk-nodes";
    try {
      let added = 0;
      for (let i = 0; i < unregistered.length; i++) {
        await createNodeForAgent(unregistered[i], i);
        added++;
      }
      onRefresh();
      showAlert({
        title: $_("report.twagentNodeAddedTitle") || "マップにノードを追加しました",
        message: $_("report.twagentBulkNodesAddedMsg", { values: { count: added } }) ||
          `${added} 台のエージェントノードをマップに追加しました。`,
        type: "info",
      });
    } catch (err: any) {
      showAlert({
        title: $_("common.error") || "エラー",
        message: err.message || String(err),
        type: "danger",
      });
    } finally {
      actionBusyId = null;
    }
  };

  const getProtocols = (ent: { Protocols?: string[]; Protocol?: string }): string[] => {
    if (ent.Protocols && ent.Protocols.length > 0) {
      return ent.Protocols;
    }
    if (ent.Protocol) {
      return ent.Protocol.split(",").map((s) => s.trim()).filter(Boolean);
    }
    return ["mqtt"];
  };

  // One-click Recommended Pollings Creation (creates node automatically if needed)
  const handleQuickAddPollings = async (agent: TwAgentInfoEnt, e?: Event) => {
    e?.stopPropagation();
    if (actionBusyId) return;
    actionBusyId = agent.ID;
    try {
      const targetNode = await createNodeForAgent(agent);
      const nodeId = targetNode.id || targetNode.ID || "";
      const existingPollings = getAgentPollings(agent, targetNode);
      const hasPoll = (type: string, filterSub: string, nameSub: string) =>
        existingPollings.some((p) => {
          const pType = (p.type || p.Type || "").toLowerCase();
          const pFilter = (p.filter || p.Filter || "").toLowerCase();
          const pName = (p.name || p.Name || "").toLowerCase();
          return pType === type && (
            (filterSub && pFilter.includes(filterSub.toLowerCase())) ||
            (nameSub && pName.includes(nameSub.toLowerCase()))
          );
        });

      const agentTopicId = agent.AgentID && agent.AgentID !== agent.IP ? agent.AgentID : "+";
      const toCreate: Partial<PollingEnt>[] = [];

      const protos = Array.isArray(agent.Protocols) ? agent.Protocols.map((p) => p.toLowerCase()) : [];
      const hasMqtt = protos.length === 0 || protos.includes("mqtt");
      const hasSyslog = protos.includes("syslog");
      const hasTrap = protos.includes("trap");

      if (hasMqtt) {
        if (!hasPoll("mqtt", "/status", "LWT")) {
          toCreate.push({
            node_id: nodeId,
            name: isJaLocale ? "twagentneo 死活監視 (MQTT LWT)" : "twagentneo Status (MQTT LWT)",
            type: "mqtt",
            mode: "twagent",
            filter: `twsnmpneo/agent/${agentTopicId}/status`,
            script: "status == 'online' && ageSec < 180",
            level: "high",
            poll_int: 60,
            timeout: 5,
            retry: 1,
          });
        }
        if (!hasPoll("mqtt", "/metrics", "リソース")) {
          toCreate.push({
            node_id: nodeId,
            name: isJaLocale ? "twagentneo リソース監視 (CPU/Mem/Disk)" : "twagentneo Resource Monitor (CPU/Mem/Disk)",
            type: "mqtt",
            mode: "twagent",
            filter: `twsnmpneo/agent/${agentTopicId}/metrics`,
            script: "cpu_percent < 90 && mem_percent < 90 && disk_used_percent < 95",
            level: "low",
            poll_int: 60,
            timeout: 5,
            retry: 1,
          });
        }
        if (!hasPoll("mqtt", "/alert", "AI脅威")) {
          toCreate.push({
            node_id: nodeId,
            name: isJaLocale ? "twagentneo AI脅威・コンプライアンス監視 (MQTT)" : "twagentneo AI Threat & Compliance (MQTT)",
            type: "mqtt",
            mode: "twagent",
            filter: `twsnmpneo/agent/${agentTopicId}/alert`,
            script: "threatCount == 0 && compCount == 0",
            level: "high",
            poll_int: 60,
            timeout: 5,
            retry: 0,
          });
        }
      }

      if (hasSyslog && !hasPoll("syslog", "", "syslog")) {
        toCreate.push({
          node_id: nodeId,
          name: isJaLocale ? "twagentneo AI脅威・FIM監視 (Syslog)" : "twagentneo AI Threat & FIM (Syslog)",
          type: "syslog",
          mode: "twagent",
          script: "threatCount == 0 && compCount == 0",
          level: "high",
          poll_int: 60,
          timeout: 5,
          retry: 0,
        });
      }

      if (hasTrap && !hasPoll("trap", "", "trap")) {
        toCreate.push({
          node_id: nodeId,
          name: isJaLocale ? "twagentneo SNMP TRAP 脅威監視" : "twagentneo SNMP TRAP Threat Monitor",
          type: "trap",
          mode: "twagent",
          script: "threatCount == 0 && compCount == 0",
          level: "high",
          poll_int: 60,
          timeout: 5,
          retry: 0,
        });
      }

      for (const p of toCreate) {
        await savePolling(p);
      }

      onRefresh();
      showAlert({
        title: $_("report.twagentPollingsAddedTitle") || "ポーリングを追加しました",
        message: toCreate.length > 0
          ? ($_("report.twagentPollingsAddedMsg", {
              values: { name: targetNode.name || targetNode.Name, count: toCreate.length },
            }) || `ノード「${targetNode.name || targetNode.Name}」に ${toCreate.length} 件の twagentneo 監視ポーリングを追加しました。`)
          : ($_("report.twagentPollingsAlreadyExist") || "推奨ポーリングはすでに登録されています。"),
        type: "info",
      });
    } catch (err: any) {
      showAlert({
        title: $_("common.error") || "エラー",
        message: err.message || String(err),
        type: "danger",
      });
    } finally {
      actionBusyId = null;
    }
  };

  const handleOpenNodeDialog = (agent: TwAgentInfoEnt, e?: Event) => {
    e?.stopPropagation();
    const existing = findMatchedNode(agent);
    if (existing) {
      nodeToEdit = { ...existing };
    } else {
      nodeToEdit = {
        id: "",
        name: agent.Hostname || agent.AgentID || agent.IP || "twagentneo",
        ip: agent.IP || "127.0.0.1",
        mac: agent.MAC || "",
        icon: osToIcon(agent.OS),
        state: agent.Status === "online" ? "normal" : "warn",
        addr_mode: agent.IP ? "ip" : "host",
        descr: `twagentneo ${agent.Version || ""} (${agent.OS || "agent"})`.trim(),
        x: 200,
        y: 200,
      };
    }
    showNodeModal = true;
  };

  const handleOpenPollingDialog = async (agent: TwAgentInfoEnt, e?: Event) => {
    e?.stopPropagation();
    let targetNode = findMatchedNode(agent);
    if (!targetNode) {
      targetNode = await createNodeForAgent(agent);
      onRefresh();
    }
    const agentTopicId = agent.AgentID && agent.AgentID !== agent.IP ? agent.AgentID : "+";
    pollingToEdit = {
      id: "",
      node_id: targetNode.id || targetNode.ID || "",
      name: isJaLocale ? `twagentneo 監視 (${agent.Hostname || agent.AgentID})` : `twagentneo Monitor (${agent.Hostname || agent.AgentID})`,
      type: "mqtt",
      mode: "twagent",
      params: "",
      filter: `twsnmpneo/agent/${agentTopicId}/metrics`,
      extractor: "",
      script: "cpu_percent < 90 && mem_percent < 90 && disk_used_percent < 95",
      level: "low",
      poll_int: 60,
      timeout: 5,
      retry: 1,
      log_mode: 1,
      state: "unknown",
    };
    showPollingModal = true;
  };

  const copyHash = async (id: string, hash: string, e: Event) => {
    e.stopPropagation();
    if (!hash) return;
    try {
      await navigator.clipboard.writeText(hash);
      copiedHashId = id;
      setTimeout(() => {
        if (copiedHashId === id) copiedHashId = null;
      }, 1500);
    } catch (_) {}
  };

  // KPIs
  const kpis = $derived.by(() => {
    const onlineAgents = agents.filter((a) => a.Status === "online").length;
    const unregisteredAgents = agents.filter((a) => !findMatchedNode(a)).length;
    const realThreats = alerts.filter((a) => a.IsThreat).reduce((s, a) => s + (a.Count || 1), 0);
    const benignFiltered = alerts.filter((a) => a.AIJudged && !a.IsThreat).reduce((s, a) => s + (a.Count || 1), 0);
    const compViolations = alerts.filter((a) => a.ComplianceViolated).reduce((s, a) => s + (a.Count || 1), 0);
    const totalFim = fimEvents.reduce((s, f) => s + (f.Count || 1), 0);
    const totalSca = scaFindings.reduce((s, f) => s + (f.Count || 1), 0);
    const criticalSca = scaFindings.filter((f) => f.Severity === "critical" || f.Severity === "high").length;
    const riskyTcp = tcpConns.filter((t) => t.ValidScore && t.Score < 40).length;

    return {
      totalAgents: agents.length,
      onlineAgents,
      unregisteredAgents,
      realThreats,
      benignFiltered,
      compViolations,
      totalFim,
      totalSca,
      criticalSca,
      totalTcp: tcpConns.length,
      riskyTcp,
    };
  });

  // Top ECharts rendering
  const renderCharts = () => {
    const isDark = isDarkMode();

    if (resourceChartElem) {
      if (resourceChartInstance) resourceChartInstance.dispose();
      resourceChartInstance = echarts.init(resourceChartElem, isDark ? "dark" : undefined);

      const topAgents = [...agents]
        .sort((a, b) => Math.max(b.CPUPercent, b.MemPercent, b.DiskUsedPercent) - Math.max(a.CPUPercent, a.MemPercent, a.DiskUsedPercent))
        .slice(0, 6);

      const names = topAgents.map((a) => a.Hostname || a.AgentID || a.IP);
      resourceChartInstance.setOption({
        backgroundColor: "transparent",
        tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
        legend: {
          top: 0,
          textStyle: { fontSize: 10, color: isDark ? "#94a3b8" : "#64748b" },
        },
        grid: { top: 26, bottom: 22, left: 36, right: 12 },
        xAxis: {
          type: "category",
          data: names,
          axisLabel: { fontSize: 10, color: isDark ? "#cbd5e1" : "#475569" },
        },
        yAxis: {
          type: "value",
          max: 100,
          axisLabel: { fontSize: 10, formatter: "{value}%", color: isDark ? "#94a3b8" : "#64748b" },
          splitLine: { lineStyle: { color: isDark ? "#1e293b" : "#e2e8f0", type: "dashed" } },
        },
        series: [
          {
            name: "CPU %",
            type: "bar",
            data: topAgents.map((a) => Number((a.CPUPercent || 0).toFixed(1))),
            itemStyle: { color: "#06b6d4", borderRadius: [3, 3, 0, 0] },
          },
          {
            name: "Mem %",
            type: "bar",
            data: topAgents.map((a) => Number((a.MemPercent || 0).toFixed(1))),
            itemStyle: { color: "#8b5cf6", borderRadius: [3, 3, 0, 0] },
          },
          {
            name: "Disk %",
            type: "bar",
            data: topAgents.map((a) => Number((a.DiskUsedPercent || 0).toFixed(1))),
            itemStyle: { color: "#f59e0b", borderRadius: [3, 3, 0, 0] },
          },
        ],
      });
    }

    if (securityChartElem) {
      if (securityChartInstance) securityChartInstance.dispose();
      securityChartInstance = echarts.init(securityChartElem, isDark ? "dark" : undefined);

      securityChartInstance.setOption({
        backgroundColor: "transparent",
        tooltip: { trigger: "item" },
        legend: {
          orient: "vertical",
          right: 8,
          top: "middle",
          textStyle: { fontSize: 10, color: isDark ? "#cbd5e1" : "#475569" },
        },
        series: [
          {
            name: isJaLocale ? "イベント内訳" : "Security Breakdown",
            type: "pie",
            radius: ["45%", "72%"],
            center: ["32%", "52%"],
            label: { show: false },
            data: [
              { value: kpis.realThreats, name: isJaLocale ? "AI 脅威判定" : "AI Threats", itemStyle: { color: "#f43f5e" } },
              { value: kpis.compViolations, name: isJaLocale ? "コンプライアンス違反" : "Compliance", itemStyle: { color: "#f59e0b" } },
              { value: kpis.benignFiltered, name: isJaLocale ? "AI 誤検知除外" : "AI Filtered Benign", itemStyle: { color: "#10b981" } },
              { value: kpis.totalFim, name: isJaLocale ? "FIM 変更検知" : "FIM Events", itemStyle: { color: "#06b6d4" } },
            ],
          },
        ],
      });
    }
  };

  const renderDetailChart = (agent: TwAgentInfoEnt) => {
    if (!detailChartElem || !agent.Samples || agent.Samples.length === 0) return;
    if (detailChartInstance) detailChartInstance.dispose();
    const isDark = isDarkMode();
    detailChartInstance = echarts.init(detailChartElem, isDark ? "dark" : undefined);

    const times = agent.Samples.map((s) => formatTimeStr(s.Time));
    detailChartInstance.setOption({
      backgroundColor: "transparent",
      tooltip: { trigger: "axis" },
      legend: {
        top: 0,
        textStyle: { fontSize: 10, color: isDark ? "#94a3b8" : "#64748b" },
      },
      grid: { top: 28, bottom: 22, left: 40, right: 40 },
      xAxis: {
        type: "category",
        data: times,
        axisLabel: { fontSize: 9, color: isDark ? "#94a3b8" : "#64748b" },
      },
      yAxis: [
        {
          type: "value",
          max: 100,
          axisLabel: { fontSize: 9, formatter: "{value}%", color: isDark ? "#94a3b8" : "#64748b" },
          splitLine: { lineStyle: { color: isDark ? "#1e293b" : "#e2e8f0", type: "dashed" } },
        },
        {
          type: "value",
          axisLabel: { fontSize: 9, formatter: "{value}M", color: isDark ? "#94a3b8" : "#64748b" },
          splitLine: { show: false },
        },
      ],
      series: [
        {
          name: "CPU %",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: agent.Samples.map((s) => Number((s.CPU || 0).toFixed(1))),
          itemStyle: { color: "#06b6d4" },
        },
        {
          name: "Mem %",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: agent.Samples.map((s) => Number((s.Mem || 0).toFixed(1))),
          itemStyle: { color: "#8b5cf6" },
        },
        {
          name: "Rx Mbps",
          type: "line",
          yAxisIndex: 1,
          smooth: true,
          showSymbol: false,
          data: agent.Samples.map((s) => Number((s.RxMbps || 0).toFixed(2))),
          itemStyle: { color: "#10b981" },
        },
        {
          name: "Tx Mbps",
          type: "line",
          yAxisIndex: 1,
          smooth: true,
          showSymbol: false,
          data: agent.Samples.map((s) => Number((s.TxMbps || 0).toFixed(2))),
          itemStyle: { color: "#f59e0b" },
        },
      ],
    });
  };

  $effect(() => {
    if ((agents.length > 0 || alerts.length > 0 || fimEvents.length > 0) && (resourceChartElem || securityChartElem)) {
      tick().then(() => renderCharts());
    }
  });

  $effect(() => {
    if (activeTab === "agents" && expandedId && detailChartElem) {
      const target = agents.find((a) => a.ID === expandedId);
      if (target && target.Samples && target.Samples.length > 0) {
        tick().then(() => renderDetailChart(target));
      }
    }
  });

  onMount(() => {
    void loadAll();
    const handleResize = () => {
      resourceChartInstance?.resize();
      securityChartInstance?.resize();
      detailChartInstance?.resize();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      resourceChartInstance?.dispose();
      securityChartInstance?.dispose();
      detailChartInstance?.dispose();
    };
  });

  const handleTabChange = (tab: TabType) => {
    activeTab = tab;
    expandedId = null;
    currentPage = 1;
    sortColumn = "LastTime";
    sortDirection = "desc";
  };

  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection = "desc";
    }
  };

  const currentList = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    let res: any[] = [];
    if (activeTab === "agents") {
      res = agents;
    } else if (activeTab === "alerts") {
      res = alerts.filter((a) => {
        if (alertFilter === "threat") return a.IsThreat;
        if (alertFilter === "compliance") return a.ComplianceViolated;
        if (alertFilter === "benign") return a.AIJudged && !a.IsThreat;
        return true;
      });
    } else if (activeTab === "fim") {
      res = fimEvents;
    } else if (activeTab === "sca") {
      res = scaFindings;
    } else if (activeTab === "tcp") {
      res = tcpConns;
    }

    if (q) {
      res = res.filter((item) =>
        Object.values(item).some((v) => {
          if (typeof v === "string") return v.toLowerCase().includes(q);
          if (Array.isArray(v)) return v.join(" ").toLowerCase().includes(q);
          return false;
        })
      );
    }

    return [...res].sort((a, b) => {
      let valA = a[sortColumn];
      let valB = b[sortColumn];
      if (typeof valA === "string") {
        const cmp = valA.localeCompare(valB || "");
        return sortDirection === "asc" ? cmp : -cmp;
      }
      valA = Number(valA || 0);
      valB = Number(valB || 0);
      return sortDirection === "asc" ? valA - valB : valB - valA;
    });
  });

  const paginatedList = $derived.by(() => {
    if (pageSize === -1) return currentList;
    const start = (currentPage - 1) * pageSize;
    return currentList.slice(start, start + pageSize);
  });

  export function exportCSV(): void {
    let header = "";
    let rows = "";
    if (activeTab === "agents") {
      header = "AgentID,Hostname,IP,MAC,OS,Version,Status,CPUPercent,MemPercent,DiskUsedPercent,RxMbps,TxMbps,ThreatCount,FIMCount,LastTime\n";
      rows = (currentList as TwAgentInfoEnt[])
        .map((a) => `"${a.AgentID}","${a.Hostname}","${a.IP}","${a.MAC}","${a.OS}","${a.Version}","${a.Status}","${a.CPUPercent.toFixed(1)}","${a.MemPercent.toFixed(1)}","${a.DiskUsedPercent.toFixed(1)}","${a.RxMbps.toFixed(2)}","${a.TxMbps.toFixed(2)}","${a.ThreatCount}","${a.FIMCount}","${formatTimeStr(a.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "alerts") {
      header = "Level,IsThreat,Confidence,ComplianceViolated,Compliance,Protocol,Host,Category,RuleTitle,SummaryJa,ActionJa,Count,LastTime\n";
      rows = (currentList as TwAgentAlertEnt[])
        .map((a) => `"${a.Level}","${a.IsThreat}","${(a.Confidence * 100).toFixed(0)}%","${a.ComplianceViolated}","${(a.ComplianceTypes || []).join("|")}","${getProtocols(a).join(", ")}","${a.Host}","${a.Category}","${(a.RuleTitle || "").replace(/"/g, '""')}","${(a.SummaryJa || "").replace(/"/g, '""')}","${(a.ActionJa || "").replace(/"/g, '""')}","${a.Count}","${formatTimeStr(a.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "fim") {
      header = "Op,Path,Hash,Host,Protocol,Count,LastTime\n";
      rows = (currentList as TwAgentFIMEnt[])
        .map((f) => `"${f.Op}","${f.Path}","${f.Hash}","${f.Host}","${getProtocols(f).join(", ")}","${f.Count}","${formatTimeStr(f.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "sca") {
      header = "Severity,RuleID,Title,FilePath,Proof,Remediation,Compliance,IsThreat,ComplianceViolated,Protocol,Host,Count,LastTime\n";
      rows = (currentList as TwAgentSCAEnt[])
        .map((s) => `"${s.Severity}","${s.RuleID || ""}","${(s.Title || "").replace(/"/g, '""')}","${(s.FilePath || "").replace(/"/g, '""')}","${(s.Proof || "").replace(/"/g, '""')}","${(s.Remediation || "").replace(/"/g, '""')}","${(s.Compliance || []).join("|")}","${s.IsThreat}","${s.ComplianceViolated}","${getProtocols(s).join(", ")}","${s.Host}","${s.Count}","${formatTimeStr(s.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "tcp") {
      header = "Score,Host,Process,RemoteIP,RemoteLoc,Services,ConnCount,Count,LastTime\n";
      rows = (currentList as TwAgentTCPEnt[])
        .map((t) => `"${t.Score?.toFixed(1) || ""}","${t.Host}","${t.Process}","${t.RemoteIP}","${t.RemoteLoc}","${(t.Services || []).join("|")}","${t.ConnCount}","${t.Count}","${formatTimeStr(t.LastTime)}"`)
        .join("\n");
    }

    const blob = new Blob(["\uFEFF" + header + rows], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_twagent_${activeTab}_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-5">
  <!-- Header & Actions -->
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div>
      <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Bot class="w-5 h-5 text-cyan-500 dark:text-cyan-400" />
        <span>{$_("report.twagentTitle") || "twagentneo エージェント監視・AIセキュリティ分析"}</span>
      </h2>
      <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
        {$_("report.twagentSubtitle") || "MQTT / Syslog (RFC 5424) / SNMP TRAP から自動集計されたエージェント稼働・AI脅威判定・FIM・TCP通信レポート"}
      </p>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      {#if kpis.unregisteredAgents > 0}
        <button
          type="button"
          disabled={actionBusyId !== null}
          onclick={handleBulkAddUnregisteredNodes}
          class="flex items-center gap-1.5 rounded-xl border border-emerald-500/40 bg-emerald-500/10 hover:bg-emerald-500/20 px-3.5 py-1.5 text-xs font-bold text-emerald-700 dark:text-emerald-300 shadow-xs transition-all cursor-pointer disabled:opacity-50"
        >
          <Plus class="w-4 h-4 text-emerald-500" />
          <span>
            {$_("report.twagentBtnBulkAddNodes", { values: { count: kpis.unregisteredAgents } }) ||
              `未登録ノードを一括追加 (${kpis.unregisteredAgents}台)`}
          </span>
        </button>
      {/if}

      <button
        type="button"
        onclick={() => (showAiExplainModal = true)}
        class="flex items-center gap-1.5 rounded-xl border border-cyan-500/40 bg-cyan-500/10 hover:bg-cyan-500/20 px-3.5 py-1.5 text-xs font-bold text-cyan-600 dark:text-cyan-400 shadow-xs transition-all cursor-pointer"
      >
        <Sparkles class="w-4 h-4 text-cyan-500" />
        <span>{$_("report.btnAiTwagent") || "AI エージェント総合診断"}</span>
      </button>
    </div>
  </div>

  <!-- Top KPI Cards + ECharts Summary Row -->
  <div class="grid grid-cols-1 lg:grid-cols-12 gap-4">
    <!-- 4 KPI Cards (6 cols) -->
    <div class="lg:col-span-6 grid grid-cols-1 sm:grid-cols-2 gap-3">
      <!-- KPI 1: Agents -->
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
        <div>
          <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">
            {$_("report.twagentKpiAgents") || "稼働エージェント (Online / 総数)"}
          </span>
          <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400 mt-1">
            {kpis.onlineAgents} <span class="text-sm text-slate-400">/ {kpis.totalAgents}</span>
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
            {#if kpis.unregisteredAgents > 0}
              <span class="text-amber-600 dark:text-amber-400 font-semibold">
                {$_("report.twagentUnregisteredCount", { values: { count: kpis.unregisteredAgents } }) || `マップ未登録: ${kpis.unregisteredAgents}台`}
              </span>
            {:else}
              <span class="text-emerald-600 dark:text-emerald-400">
                {$_("report.twagentAllRegistered") || "すべてマップ登録済み"}
              </span>
            {/if}
          </div>
        </div>
        <div class="h-10 w-10 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-600 dark:text-cyan-400">
          <Bot class="w-5 h-5" />
        </div>
      </div>

      <!-- KPI 2: AI Real Threats vs Filtered Benign -->
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
        <div>
          <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">
            {$_("report.twagentKpiThreats") || "AI 脅威判定 (tensai)"}
          </span>
          <div class="text-2xl font-bold font-mono {kpis.realThreats > 0 ? 'text-rose-600 dark:text-rose-400' : 'text-emerald-600 dark:text-emerald-400'} mt-1">
            {kpis.realThreats.toLocaleString()}
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
            {$_("report.twagentBenignFiltered", { values: { count: kpis.benignFiltered } }) || `AI誤検知除外: ${kpis.benignFiltered}件`}
          </div>
        </div>
        <div class="h-10 w-10 rounded-xl {kpis.realThreats > 0 ? 'bg-rose-500/10 border-rose-500/30 text-rose-500' : 'bg-emerald-500/10 border-emerald-500/30 text-emerald-500'} border flex items-center justify-center">
          <ShieldAlert class="w-5 h-5" />
        </div>
      </div>

      <!-- KPI 3: Compliance Violations -->
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
        <div>
          <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">
            {$_("report.twagentKpiCompliance") || "コンプライアンス違反"}
          </span>
          <div class="text-2xl font-bold font-mono {kpis.compViolations > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-slate-700 dark:text-slate-200'} mt-1">
            {kpis.compViolations.toLocaleString()}
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
            PCI-DSS / CIS / HIPAA / GDPR
          </div>
        </div>
        <div class="h-10 w-10 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-center justify-center text-amber-600 dark:text-amber-400">
          <ShieldCheck class="w-5 h-5" />
        </div>
      </div>

      <!-- KPI 4: FIM & TCP -->
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
        <div>
          <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">
            {$_("report.twagentKpiFimTcp") || "FIM 変更検知 / プロセス通信"}
          </span>
          <div class="text-2xl font-bold font-mono text-indigo-600 dark:text-indigo-400 mt-1">
            {kpis.totalFim.toLocaleString()} <span class="text-sm text-slate-400">/ {kpis.totalTcp}</span>
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
            {$_("report.twagentRiskyTcpCount", { values: { count: kpis.riskyTcp } }) || `要注意外部通信: ${kpis.riskyTcp}件`}
          </div>
        </div>
        <div class="h-10 w-10 rounded-xl bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-600 dark:text-indigo-400">
          <FileSpreadsheet class="w-5 h-5" />
        </div>
      </div>
    </div>

    <!-- ECharts Resource Chart (3 cols) -->
    <div class="lg:col-span-3 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3 shadow-sm flex flex-col justify-between h-64">
      <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/60 pb-1.5">
        <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">
          {$_("report.twagentChartResources") || "エージェント リソース使用率"}
        </span>
        <span class="text-[10px] font-mono text-slate-400">CPU/Mem/Disk</span>
      </div>
      <div bind:this={resourceChartElem} class="h-52 w-full"></div>
    </div>

    <!-- ECharts Security Breakdown (3 cols) -->
    <div class="lg:col-span-3 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3 shadow-sm flex flex-col justify-between h-64">
      <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/60 pb-1.5">
        <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">
          {$_("report.twagentChartSecurity") || "AIセキュリティ & FIM 内訳"}
        </span>
        <span class="text-[10px] font-mono text-slate-400">tensai AI</span>
      </div>
      <div bind:this={securityChartElem} class="h-52 w-full"></div>
    </div>
  </div>

  <!-- Sub-tab Selector Pills -->
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="flex flex-wrap items-center gap-2 p-1.5 rounded-2xl bg-slate-100 dark:bg-slate-900 border border-slate-200 dark:border-slate-800">
      <button
        type="button"
        onclick={() => handleTabChange("agents")}
        class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'agents' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
      >
        <Bot class="w-3.5 h-3.5" />
        <span>{$_("report.twagentTabAgents") || "エージェント一覧"} ({agents.length})</span>
      </button>
      <button
        type="button"
        onclick={() => handleTabChange("alerts")}
        class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'alerts' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
      >
        <ShieldAlert class="w-3.5 h-3.5" />
        <span>{$_("report.twagentTabAlerts") || "AIセキュリティアラート"} ({alerts.length})</span>
      </button>
      <button
        type="button"
        onclick={() => handleTabChange("fim")}
        class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'fim' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
      >
        <FileSpreadsheet class="w-3.5 h-3.5" />
        <span>{$_("report.twagentTabFim") || "ファイル整合性監視 (FIM)"} ({fimEvents.length})</span>
      </button>
      <button
        type="button"
        onclick={() => handleTabChange("sca")}
        class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'sca' ? 'bg-amber-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
      >
        <SlidersHorizontal class="w-3.5 h-3.5" />
        <span>{$_("report.twagentTabSca") || "構成監査 (SCA)"} ({scaFindings.length})</span>
      </button>
      <button
        type="button"
        onclick={() => handleTabChange("tcp")}
        class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'tcp' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
      >
        <Network class="w-3.5 h-3.5" />
        <span>{$_("report.twagentTabTcp") || "プロセスTCP通信"} ({tcpConns.length})</span>
      </button>
    </div>

    {#if activeTab === "alerts"}
      <div class="flex flex-wrap items-center gap-1.5">
        <button
          type="button"
          onclick={() => (alertFilter = "all")}
          class="px-3 py-1 rounded-xl text-xs font-semibold cursor-pointer transition-all {alertFilter === 'all' ? 'bg-slate-700 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400'}"
        >
          {$_("report.twagentFilterAll") || "すべて"} ({alerts.length})
        </button>
        <button
          type="button"
          onclick={() => (alertFilter = "threat")}
          class="px-3 py-1 rounded-xl text-xs font-semibold cursor-pointer transition-all {alertFilter === 'threat' ? 'bg-rose-600 text-white' : 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/30'}"
        >
          {$_("report.twagentFilterThreat") || "AI脅威判定のみ"} ({alerts.filter(a => a.IsThreat).length})
        </button>
        <button
          type="button"
          onclick={() => (alertFilter = "compliance")}
          class="px-3 py-1 rounded-xl text-xs font-semibold cursor-pointer transition-all {alertFilter === 'compliance' ? 'bg-amber-600 text-white' : 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/30'}"
        >
          {$_("report.twagentFilterCompliance") || "コンプライアンス違反"} ({alerts.filter(a => a.ComplianceViolated).length})
        </button>
        <button
          type="button"
          onclick={() => (alertFilter = "benign")}
          class="px-3 py-1 rounded-xl text-xs font-semibold cursor-pointer transition-all {alertFilter === 'benign' ? 'bg-emerald-600 text-white' : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30'}"
        >
          {$_("report.twagentFilterBenign") || "AI誤検知除外 (良性)"} ({alerts.filter(a => a.AIJudged && !a.IsThreat).length})
        </button>
      </div>
    {/if}
  </div>

  <!-- Main Table Card -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm overflow-hidden">
    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs border-collapse font-sans">
        <thead class="bg-slate-50 dark:bg-slate-950/80 border-b border-slate-200 dark:border-slate-800 text-slate-600 dark:text-slate-400 font-semibold select-none">
          <tr>
            <th class="py-2.5 px-2.5 w-7"></th>
            {#if activeTab === "agents"}
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Status")}>{$_("report.colState") || "状態"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Hostname")}>{$_("report.twagentColHostname") || "ホスト名 / Agent ID"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("IP")}>{$_("report.colIp") || "IPアドレス"}</th>
              <th class="py-2.5 px-2.5">{$_("report.twagentColMapStatus") || "マップ登録状態"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("OS")}>OS / Ver</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("CPUPercent")}>CPU %</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("MemPercent")}>Mem %</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("DiskUsedPercent")}>Disk %</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("RxMbps")}>Rx / Tx (Mbps)</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("ThreatCount")}>{$_("report.twagentColThreatsFim") || "脅威 / FIM / SCA"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.colLastTime") || "最終受信"}</th>
              <th class="py-2.5 px-2.5 text-right">{$_("report.twagentColQuickActions") || "ワンクリック追加 (ノード / ポーリング)"}</th>
            {:else if activeTab === "alerts"}
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Level")}>{$_("report.twagentColLevel") || "レベル"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("IsThreat")}>{$_("report.twagentColAiVerdict") || "AI判定 (tensai)"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Host")}>{$_("report.twagentColHost") || "ホスト"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Protocol")}>{$_("report.twagentColProtocol") || "プロトコル"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Category")}>{$_("report.twagentColCategory") || "カテゴリ"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("RuleTitle")}>{$_("report.twagentColRuleTitle") || "検知ルール / AI要約"}</th>
              <th class="py-2.5 px-2.5">{$_("report.twagentColCompliance") || "コンプライアンス"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.winColCount") || "件数"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.colLastTime") || "最終検知"}</th>
            {:else if activeTab === "fim"}
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Op")}>{$_("report.twagentColFimOp") || "操作"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Path")}>{$_("report.twagentColFimPath") || "監視ファイルパス"}</th>
              <th class="py-2.5 px-2.5">{$_("report.twagentColFimHash") || "SHA256 ハッシュ"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Host")}>{$_("report.twagentColHost") || "ホスト"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Protocol")}>{$_("report.twagentColProtocol") || "経路"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.winColCount") || "回数"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.colLastTime") || "最終検知"}</th>
            {:else if activeTab === "sca"}
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Severity")}>{$_("report.twagentColLevel") || "深刻度"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("RuleID")}>{$_("report.twagentColScaRule") || "ルール"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("FilePath")}>{$_("report.twagentColScaPath") || "対象ファイル"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Host")}>{$_("report.twagentColHost") || "ホスト"}</th>
              <th class="py-2.5 px-2.5 text-center">{$_("report.twagentColAiVerdict") || "AI判定"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Protocol")}>{$_("report.twagentColProtocol") || "経路"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.winColCount") || "回数"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.colLastTime") || "最終検知"}</th>
            {:else if activeTab === "tcp"}
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Score")}>{$_("report.winColTrustScore") || "信用スコア"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Host")}>{$_("report.twagentColHost") || "ホスト"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Process")}>{$_("report.winColProcessName") || "プロセス名"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("RemoteIP")}>{$_("report.twagentColRemoteIp") || "通信先IP"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("RemoteLoc")}>{$_("report.twagentColLocation") || "国・地域"}</th>
              <th class="py-2.5 px-2.5">{$_("report.winColService") || "サービス"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("ConnCount")}>{$_("report.twagentColActiveConns") || "接続数 (累計)"}</th>
              <th class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.colLastTime") || "最終確認"}</th>
            {/if}
          </tr>
        </thead>

        <tbody class="divide-y divide-slate-200 dark:divide-slate-800 font-mono text-[11px]">
          {#if paginatedList.length === 0}
            <tr>
              <td colspan="13" class="py-10 text-center text-slate-400 font-sans">
                {internalLoading ? $_("report.loading") : ($_("report.twagentNoData") || "twagentneo の受信データはまだありません（MQTT / Syslog / SNMP TRAP 受信時に自動生成されます）")}
              </td>
            </tr>
          {:else}
            {#each paginatedList as item}
              {@const isExpanded = expandedId === item.ID}
              {#if activeTab === "agents"}
                {@const a = item as TwAgentInfoEnt}
                {@const matchedNode = findMatchedNode(a)}
                {@const agentPollings = getAgentPollings(a, matchedNode)}
                <tr
                  class="hover:bg-cyan-50/40 dark:hover:bg-cyan-950/20 transition-colors cursor-pointer {isExpanded ? 'bg-cyan-50/30 dark:bg-cyan-950/15' : ''}"
                  onclick={() => (expandedId = isExpanded ? null : a.ID)}
                >
                  <td class="py-1 px-2 text-center text-slate-400">
                    {#if isExpanded}
                      <ChevronDown class="w-4 h-4 text-cyan-500" />
                    {:else}
                      <ChevronRight class="w-4 h-4" />
                    {/if}
                  </td>
                  <td class="py-1 px-2 whitespace-nowrap">
                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold {a.Status === 'online' ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border border-emerald-500/30' : 'bg-rose-500/15 text-rose-700 dark:text-rose-300 border border-rose-500/30'}">
                      <span class="w-1.5 h-1.5 rounded-full {a.Status === 'online' ? 'bg-emerald-500' : 'bg-rose-500'}"></span>
                      {a.Status || "online"}
                    </span>
                  </td>
                  <td class="py-1 px-2 font-sans">
                    <div class="font-bold text-slate-800 dark:text-slate-100">{a.Hostname || a.AgentID}</div>
                    {#if a.AgentID && a.AgentID !== a.Hostname}
                      <div class="text-[10px] font-mono text-slate-400">ID: {a.AgentID}</div>
                    {/if}
                  </td>
                  <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 whitespace-nowrap">
                    <div>{a.IP || "-"}</div>
                    {#if a.MAC}
                      <div class="text-[10px] text-slate-400">{a.MAC}</div>
                    {/if}
                  </td>
                  <td class="py-1 px-2 font-sans whitespace-nowrap">
                    {#if matchedNode}
                      <div class="inline-flex items-center gap-1 px-2 py-0.5 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-700 dark:text-emerald-300 text-[10px] font-semibold">
                        <CheckCircle2 class="w-3 h-3 text-emerald-500" />
                        <span>{matchedNode.name || matchedNode.Name}</span>
                      </div>
                      <div class="text-[10px] text-slate-400 mt-0.5">
                        {$_("report.twagentPollingCount", { values: { count: agentPollings.length } }) || `ポーリング: ${agentPollings.length}件`}
                      </div>
                    {:else}
                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-lg bg-amber-500/15 border border-amber-500/30 text-amber-700 dark:text-amber-300 text-[10px] font-semibold">
                        <AlertTriangle class="w-3 h-3 text-amber-500" />
                        <span>{$_("report.twagentUnregistered") || "マップ未登録"}</span>
                      </span>
                    {/if}
                  </td>
                  <td class="py-1 px-2 whitespace-nowrap">
                    <span class="px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 text-[10px] font-semibold">
                      {a.OS || "linux"}
                    </span>
                    {#if a.Version}
                      <span class="text-[10px] text-slate-400 ml-1">{a.Version}</span>
                    {/if}
                  </td>
                  <td class="py-1 px-2 text-right font-bold {a.CPUPercent >= 90 ? 'text-rose-500' : a.CPUPercent >= 75 ? 'text-amber-500' : 'text-slate-700 dark:text-slate-200'}">
                    {(a.CPUPercent || 0).toFixed(1)}%
                  </td>
                  <td class="py-1 px-2 text-right font-bold {a.MemPercent >= 90 ? 'text-rose-500' : a.MemPercent >= 80 ? 'text-amber-500' : 'text-slate-700 dark:text-slate-200'}">
                    {(a.MemPercent || 0).toFixed(1)}%
                  </td>
                  <td class="py-1 px-2 text-right font-bold {a.DiskUsedPercent >= 95 ? 'text-rose-500' : a.DiskUsedPercent >= 85 ? 'text-amber-500' : 'text-slate-700 dark:text-slate-200'}">
                    {(a.DiskUsedPercent || 0).toFixed(1)}%
                  </td>
                  <td class="py-1 px-2 text-right whitespace-nowrap">
                    <span class="text-emerald-600 dark:text-emerald-400">↓{(a.RxMbps || 0).toFixed(2)}</span>
                    <span class="text-slate-400 mx-0.5">/</span>
                    <span class="text-amber-600 dark:text-amber-400">↑{(a.TxMbps || 0).toFixed(2)}</span>
                  </td>
                  <td class="py-1 px-2 text-right whitespace-nowrap">
                    <span class="{a.ThreatCount > 0 ? 'text-rose-600 dark:text-rose-400 font-bold' : 'text-slate-400'}">
                      {a.ThreatCount || 0}
                    </span>
                    <span class="text-slate-400 mx-0.5">/</span>
                    <span class="text-cyan-600 dark:text-cyan-400">{a.FIMCount || 0}</span>
                    <span class="text-slate-400 mx-0.5">/</span>
                    <span class="{a.SCACount && a.SCACount > 0 ? 'text-amber-600 dark:text-amber-400 font-bold' : 'text-slate-400'}">{a.SCACount || 0}</span>
                  </td>
                  <td class="py-1 px-2 text-slate-500 whitespace-nowrap">{formatTimeStr(a.LastTime)}</td>
                  <td class="py-1 px-2 text-right whitespace-nowrap font-sans" onclick={(e) => e.stopPropagation()}>
                    <div class="inline-flex items-center justify-end gap-1.5">
                      {#if !matchedNode}
                        <button
                          type="button"
                          disabled={actionBusyId === a.ID}
                          onclick={(e) => handleQuickAddNode(a, e)}
                          class="inline-flex items-center gap-1 rounded-lg bg-emerald-500/15 hover:bg-emerald-500/25 border border-emerald-500/30 px-2.5 py-1 text-[11px] font-bold text-emerald-700 dark:text-emerald-300 transition-colors cursor-pointer disabled:opacity-50"
                          title={$_("report.twagentBtnAddNodeTooltip") || "ワンクリックでこのエージェントをマップにノード登録します"}
                        >
                          <Plus class="w-3.5 h-3.5" />
                          <span>{$_("report.twagentBtnAddNode") || "ノード追加"}</span>
                        </button>
                      {/if}
                      <button
                        type="button"
                        disabled={actionBusyId === a.ID}
                        onclick={(e) => handleQuickAddPollings(a, e)}
                        class="inline-flex items-center gap-1 rounded-lg bg-cyan-500/15 hover:bg-cyan-500/25 border border-cyan-500/30 px-2.5 py-1 text-[11px] font-bold text-cyan-700 dark:text-cyan-300 transition-colors cursor-pointer disabled:opacity-50"
                        title={$_("report.twagentBtnAddPollingsTooltip") || "未登録の場合はノードを作成した上で、推奨ポーリング（MQTT死活・リソース・AI脅威・Syslog・TRAP）を一括追加します"}
                      >
                        <Zap class="w-3.5 h-3.5" />
                        <span>{$_("report.twagentBtnAddPollings") || "ポーリング一括追加"}</span>
                      </button>
                      <button
                        type="button"
                        onclick={(e) => handleOpenPollingDialog(a, e)}
                        class="inline-flex items-center rounded-lg bg-slate-500/10 hover:bg-slate-500/20 border border-slate-500/30 p-1 text-slate-600 dark:text-slate-300 transition-colors cursor-pointer"
                        title={$_("report.twagentBtnCustomPollingTooltip") || "ポーリング編集ダイアログを開いて個別に追加します"}
                      >
                        <SlidersHorizontal class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>

                {#if isExpanded}
                  <tr class="bg-slate-50/90 dark:bg-slate-950/70 font-sans">
                    <td colspan="13" class="p-4 space-y-4">
                      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
                        <!-- Per-Core CPU & Memory Card -->
                        <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-3 space-y-2">
                          <div class="flex items-center justify-between">
                            <span class="text-xs font-bold text-slate-700 dark:text-slate-200 flex items-center gap-1.5">
                              <Cpu class="w-3.5 h-3.5 text-cyan-500" />
                              <span>{$_("report.twagentDetailCpuMem") || "CPU コア別負荷 & メモリ"}</span>
                            </span>
                            <span class="text-[11px] font-mono text-slate-500">
                              Mem: {renderBytes(a.MemUsedBytes || 0)} / {renderBytes(a.MemTotalBytes || 0)} ({(a.MemPercent || 0).toFixed(1)}%)
                            </span>
                          </div>
                          {#if a.PerCoreCPU && a.PerCoreCPU.length > 0}
                            <div class="grid grid-cols-4 gap-1.5 pt-1">
                              {#each a.PerCoreCPU as coreVal, idx}
                                <div class="p-1.5 rounded-lg bg-slate-50 dark:bg-slate-950 border border-slate-200/70 dark:border-slate-800">
                                  <div class="flex items-center justify-between text-[10px] font-mono">
                                    <span class="text-slate-400">#{idx}</span>
                                    <span class="font-bold text-slate-700 dark:text-slate-200">{coreVal.toFixed(0)}%</span>
                                  </div>
                                  <div class="w-full h-1 bg-slate-200 dark:bg-slate-800 rounded-full mt-1 overflow-hidden">
                                    <div
                                      class="h-full {coreVal >= 90 ? 'bg-rose-500' : coreVal >= 70 ? 'bg-amber-500' : 'bg-cyan-500'}"
                                      style="width: {Math.min(100, Math.max(0, coreVal))}%"
                                    ></div>
                                  </div>
                                </div>
                              {/each}
                            </div>
                          {:else}
                            <div class="text-xs text-slate-400 py-2">CPU Total: {(a.CPUPercent || 0).toFixed(1)}%</div>
                          {/if}
                        </div>

                        <!-- Disk Mounts Card -->
                        <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-3 space-y-2">
                          <div class="flex items-center justify-between">
                            <span class="text-xs font-bold text-slate-700 dark:text-slate-200 flex items-center gap-1.5">
                              <HardDrive class="w-3.5 h-3.5 text-amber-500" />
                              <span>{$_("report.twagentDetailDisks") || "ディスクマウント使用率"}</span>
                            </span>
                            <span class="text-[11px] font-mono text-slate-500">
                              Max: {(a.DiskUsedPercent || 0).toFixed(1)}%
                            </span>
                          </div>
                          {#if a.Disks && a.Disks.length > 0}
                            <div class="space-y-1.5 max-h-36 overflow-y-auto pr-1">
                              {#each a.Disks as d}
                                <div class="space-y-0.5">
                                  <div class="flex items-center justify-between text-[11px] font-mono">
                                    <span class="text-slate-700 dark:text-slate-300 truncate max-w-[180px]" title={d.MountPoint}>{d.MountPoint}</span>
                                    <span class="text-slate-500">{d.UsedPercent.toFixed(1)}% ({renderBytes(d.TotalBytes || 0)})</span>
                                  </div>
                                  <div class="w-full h-1.5 bg-slate-200 dark:bg-slate-800 rounded-full overflow-hidden">
                                    <div
                                      class="h-full {d.UsedPercent >= 95 ? 'bg-rose-500' : d.UsedPercent >= 85 ? 'bg-amber-500' : 'bg-emerald-500'}"
                                      style="width: {Math.min(100, Math.max(0, d.UsedPercent))}%"
                                    ></div>
                                  </div>
                                </div>
                              {/each}
                            </div>
                          {:else}
                            <div class="text-xs text-slate-400 py-2">Disk Used: {(a.DiskUsedPercent || 0).toFixed(1)}%</div>
                          {/if}
                        </div>

                        <!-- Network Interfaces Card -->
                        <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-3 space-y-2">
                          <div class="flex items-center justify-between">
                            <span class="text-xs font-bold text-slate-700 dark:text-slate-200 flex items-center gap-1.5">
                              <Activity class="w-3.5 h-3.5 text-emerald-500" />
                              <span>{$_("report.twagentDetailInterfaces") || "ネットワークインターフェース"}</span>
                            </span>
                            <button
                              type="button"
                              onclick={(e) => handleOpenNodeDialog(a, e)}
                              class="text-[11px] font-semibold text-cyan-600 dark:text-cyan-400 hover:underline cursor-pointer"
                            >
                              {$_("report.twagentOpenNodeDialog") || "ノード設定を開く"}
                            </button>
                          </div>
                          {#if a.Interfaces && a.Interfaces.length > 0}
                            <div class="space-y-1.5 max-h-36 overflow-y-auto text-[11px] font-mono">
                              {#each a.Interfaces as iface}
                                <div class="p-1.5 rounded-lg bg-slate-50 dark:bg-slate-950 border border-slate-200/70 dark:border-slate-800 flex items-center justify-between gap-2">
                                  <div>
                                    <div class="font-bold text-slate-700 dark:text-slate-200">{iface.Name} <span class="text-[10px] font-normal text-slate-400">{iface.MAC || ""}</span></div>
                                    <div class="text-[10px] text-cyan-600 dark:text-cyan-400">{(iface.Addrs || []).join(", ") || "-"}</div>
                                  </div>
                                  <div class="text-right text-[10px] text-slate-500">
                                    <div>Rx: {renderBytes(iface.BytesRecv || 0)} ({(iface.RxMbps || 0).toFixed(2)}M)</div>
                                    <div>Tx: {renderBytes(iface.BytesSent || 0)} ({(iface.TxMbps || 0).toFixed(2)}M)</div>
                                  </div>
                                </div>
                              {/each}
                            </div>
                          {:else}
                            <div class="text-xs text-slate-400 py-2">IP: {a.IP || "-"} / MAC: {a.MAC || "-"}</div>
                          {/if}
                        </div>
                      </div>

                      <!-- Time-series chart for this agent -->
                      {#if a.Samples && a.Samples.length > 0}
                        <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-3">
                          <div class="text-xs font-bold text-slate-700 dark:text-slate-200 mb-1">
                            {$_("report.twagentDetailHistoryChart") || "リソース & トラフィック推移"}
                          </div>
                          <div bind:this={detailChartElem} class="h-44 w-full"></div>
                        </div>
                      {/if}
                    </td>
                  </tr>
                {/if}

              {:else if activeTab === "alerts"}
                {@const al = item as TwAgentAlertEnt}
                <tr
                  class="hover:bg-cyan-50/40 dark:hover:bg-cyan-950/20 transition-colors cursor-pointer {isExpanded ? 'bg-cyan-50/30 dark:bg-cyan-950/15' : ''}"
                  onclick={() => (expandedId = isExpanded ? null : al.ID)}
                >
                  <td class="py-1 px-2 text-center text-slate-400">
                    {#if isExpanded}
                      <ChevronDown class="w-4 h-4 text-cyan-500" />
                    {:else}
                      <ChevronRight class="w-4 h-4" />
                    {/if}
                  </td>
                  <td class="py-1 px-2 whitespace-nowrap">
                    <span class="px-2 py-0.5 rounded-full text-[10px] font-bold {al.Level === 'CRITICAL' || al.Level === 'HIGH' ? 'bg-rose-500/15 text-rose-700 dark:text-rose-300 border border-rose-500/30' : al.Level === 'MEDIUM' ? 'bg-amber-500/15 text-amber-700 dark:text-amber-300 border border-amber-500/30' : 'bg-slate-500/15 text-slate-700 dark:text-slate-300'}">
                      {al.Level || "INFO"}
                    </span>
                  </td>
                  <td class="py-1 px-2 whitespace-nowrap font-sans">
                    {#if al.IsThreat}
                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-rose-500/15 border border-rose-500/30 text-rose-700 dark:text-rose-300 text-[10px] font-bold">
                        <ShieldAlert class="w-3 h-3 text-rose-500" />
                        <span>{$_("report.twagentVerdictThreat") || "脅威"} ({(al.Confidence * 100).toFixed(0)}%)</span>
                      </span>
                    {:else if al.AIJudged}
                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-emerald-500/15 border border-emerald-500/30 text-emerald-700 dark:text-emerald-300 text-[10px] font-bold">
                        <ShieldCheck class="w-3 h-3 text-emerald-500" />
                        <span>{$_("report.twagentVerdictBenign") || "良性・誤検知除外"} ({(al.Confidence * 100).toFixed(0)}%)</span>
                      </span>
                    {:else}
                      <span class="text-slate-400 text-[10px]">-</span>
                    {/if}
                  </td>
                  <td class="py-1 px-2 font-semibold text-slate-800 dark:text-slate-200">{al.Host}</td>
                  <td class="py-1 px-2 font-sans whitespace-nowrap">
                    <div class="flex flex-wrap items-center gap-1">
                      {#each getProtocols(al) as proto}
                        {@const pLower = proto.toLowerCase()}
                        <span class="px-1.5 py-0.5 rounded text-[10px] font-bold uppercase
                          {pLower === 'mqtt' ? 'bg-cyan-500/15 text-cyan-700 dark:text-cyan-300 border border-cyan-500/30' :
                           pLower === 'syslog' ? 'bg-blue-500/15 text-blue-700 dark:text-blue-300 border border-blue-500/30' :
                           pLower === 'trap' ? 'bg-purple-500/15 text-purple-700 dark:text-purple-300 border border-purple-500/30' :
                           'bg-slate-500/15 text-slate-700 dark:text-slate-300 border border-slate-500/30'}">
                          {proto}
                        </span>
                      {/each}
                    </div>
                  </td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400">{al.Category}</td>
                  <td class="py-1 px-2 font-sans max-w-[320px]">
                    <div class="font-semibold text-slate-800 dark:text-slate-100 truncate" title={al.RuleTitle}>{al.RuleTitle}</div>
                    {#if al.SummaryJa}
                      <div class="text-[11px] text-slate-500 dark:text-slate-400 truncate" title={al.SummaryJa}>{al.SummaryJa}</div>
                    {/if}
                  </td>
                  <td class="py-1 px-2 font-sans">
                    {#if al.ComplianceTypes && al.ComplianceTypes.length > 0}
                      <div class="flex flex-wrap gap-1">
                        {#each al.ComplianceTypes as comp}
                          <span class="px-1.5 py-0.5 rounded bg-amber-500/15 border border-amber-500/30 text-amber-700 dark:text-amber-300 text-[10px] font-bold">
                            {comp}
                          </span>
                        {/each}
                      </div>
                    {:else if al.ComplianceViolated}
                      <span class="px-1.5 py-0.5 rounded bg-amber-500/15 text-amber-700 dark:text-amber-300 text-[10px] font-bold">VIOLATED</span>
                    {:else}
                      <span class="text-slate-400">-</span>
                    {/if}
                  </td>
                  <td class="py-1 px-2 text-right font-bold text-slate-700 dark:text-slate-300">{al.Count.toLocaleString()}</td>
                  <td class="py-1 px-2 text-slate-500 whitespace-nowrap">{formatTimeStr(al.LastTime)}</td>
                </tr>

                {#if isExpanded}
                  <tr class="bg-slate-50/90 dark:bg-slate-950/70 font-sans">
                    <td colspan="10" class="p-4 space-y-3">
                      <div class="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
                        <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                          <div class="text-[11px] font-bold text-cyan-600 dark:text-cyan-400">
                            {$_("report.twagentAiSummary") || "tensai AI 分析サマリー"}
                          </div>
                          <div class="text-slate-800 dark:text-slate-200 leading-relaxed">
                            {al.SummaryJa || al.RuleTitle || "-"}
                          </div>
                        </div>
                        <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                          <div class="text-[11px] font-bold text-emerald-600 dark:text-emerald-400">
                            {$_("report.twagentAiAction") || "推奨アクション (Recommended Action)"}
                          </div>
                          <div class="text-slate-800 dark:text-slate-200 leading-relaxed">
                            {al.ActionJa || "-"}
                          </div>
                        </div>
                      </div>
                      {#if al.SourceLog}
                        <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                          <div class="text-[11px] font-bold text-slate-400 uppercase">Source Log</div>
                          <div class="font-mono text-[11px] text-slate-700 dark:text-slate-300 break-all">{al.SourceLog}</div>
                        </div>
                      {/if}
                    </td>
                  </tr>
                {/if}

              {:else if activeTab === "fim"}
                {@const f = item as TwAgentFIMEnt}
                <tr class="hover:bg-cyan-50/40 dark:hover:bg-cyan-950/20 transition-colors">
                  <td class="py-1 px-2 text-center text-slate-400">-</td>
                  <td class="py-1 px-2 whitespace-nowrap">
                    <span class="px-2 py-0.5 rounded-full text-[10px] font-bold {f.Op === 'REMOVE' ? 'bg-rose-500/15 text-rose-700 dark:text-rose-300 border border-rose-500/30' : f.Op === 'CHMOD' ? 'bg-amber-500/15 text-amber-700 dark:text-amber-300 border border-amber-500/30' : 'bg-cyan-500/15 text-cyan-700 dark:text-cyan-300 border border-cyan-500/30'}">
                      {f.Op || "WRITE"}
                    </span>
                  </td>
                  <td class="py-1 px-2 font-semibold text-slate-800 dark:text-slate-100 break-all">{f.Path}</td>
                  <td class="py-1 px-2">
                    {#if f.Hash}
                      <div class="inline-flex items-center gap-1.5">
                        <span class="text-slate-500 dark:text-slate-400 truncate max-w-[180px]" title={f.Hash}>{f.Hash}</span>
                        <button
                          type="button"
                          onclick={(e) => copyHash(f.ID, f.Hash, e)}
                          class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 text-slate-400 hover:text-cyan-500 cursor-pointer"
                          title="Copy SHA256"
                        >
                          {#if copiedHashId === f.ID}
                            <Check class="w-3 h-3 text-emerald-500" />
                          {:else}
                            <Copy class="w-3 h-3" />
                          {/if}
                        </button>
                      </div>
                    {:else}
                      <span class="text-slate-400">-</span>
                    {/if}
                  </td>
                  <td class="py-1 px-2 text-slate-700 dark:text-slate-300">{f.Host}</td>
                  <td class="py-1 px-2 font-sans whitespace-nowrap">
                    <div class="flex flex-wrap items-center gap-1">
                      {#each getProtocols(f) as proto}
                        {@const pLower = proto.toLowerCase()}
                        <span class="px-1.5 py-0.5 rounded text-[10px] font-bold uppercase
                          {pLower === 'mqtt' ? 'bg-cyan-500/15 text-cyan-700 dark:text-cyan-300 border border-cyan-500/30' :
                           pLower === 'syslog' ? 'bg-blue-500/15 text-blue-700 dark:text-blue-300 border border-blue-500/30' :
                           pLower === 'trap' ? 'bg-purple-500/15 text-purple-700 dark:text-purple-300 border border-purple-500/30' :
                           'bg-slate-500/15 text-slate-700 dark:text-slate-300 border border-slate-500/30'}">
                          {proto}
                        </span>
                      {/each}
                    </div>
                  </td>
                  <td class="py-1 px-2 text-right font-bold text-slate-700 dark:text-slate-300">{f.Count.toLocaleString()}</td>
                  <td class="py-1 px-2 text-slate-500 whitespace-nowrap">{formatTimeStr(f.LastTime)}</td>
                </tr>

              {:else if activeTab === "sca"}
                {@const sc = item as TwAgentSCAEnt}
                {@const isCrit = sc.Severity === "critical"}
                {@const isHigh = sc.Severity === "high"}
                {@const isMed = sc.Severity === "medium" || sc.Severity === "warn"}
                <tr
                  class="hover:bg-cyan-50/40 dark:hover:bg-cyan-950/20 transition-colors cursor-pointer {isExpanded ? 'bg-cyan-50/30 dark:bg-cyan-950/15' : ''}"
                  onclick={() => (expandedId = isExpanded ? null : sc.ID)}
                >
                  <td class="py-1 px-2 text-center text-slate-400">
                    {#if isExpanded}
                      <ChevronDown class="w-3.5 h-3.5 mx-auto text-cyan-600" />
                    {:else}
                      <ChevronRight class="w-3.5 h-3.5 mx-auto" />
                    {/if}
                  </td>
                  <td class="py-1 px-2 whitespace-nowrap">
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold uppercase
                      {isCrit ? 'bg-rose-500/15 text-rose-700 dark:text-rose-300 border border-rose-500/30' :
                       isHigh ? 'bg-orange-500/15 text-orange-700 dark:text-orange-300 border border-orange-500/30' :
                       isMed ? 'bg-amber-500/15 text-amber-700 dark:text-amber-300 border border-amber-500/30' :
                       'bg-slate-500/15 text-slate-700 dark:text-slate-300 border border-slate-500/30'}">
                      {sc.Severity || "info"}
                    </span>
                  </td>
                  <td class="py-1 px-2">
                    <div class="flex items-center gap-1.5">
                      {#if sc.RuleID}
                        <span class="px-1.5 py-0.5 rounded bg-slate-200 dark:bg-slate-800 text-[10px] font-mono font-bold text-slate-700 dark:text-slate-300">{sc.RuleID}</span>
                      {/if}
                      <span class="font-semibold text-slate-800 dark:text-slate-100">{sc.Title}</span>
                    </div>
                  </td>
                  <td class="py-1 px-2 font-mono text-[11px] text-slate-600 dark:text-slate-400 max-w-[240px] truncate" title={sc.FilePath}>
                    {sc.FilePath}
                  </td>
                  <td class="py-1 px-2 text-slate-700 dark:text-slate-300 whitespace-nowrap">
                    {sc.Host}
                  </td>
                  <td class="py-1 px-2 text-center whitespace-nowrap">
                    {#if sc.AIJudged}
                      <div class="inline-flex items-center gap-1">
                        {#if sc.IsThreat}
                          <span class="px-1.5 py-0.5 rounded text-[10px] font-bold bg-rose-500/15 text-rose-700 dark:text-rose-300 border border-rose-500/30">{$_("report.twagentBadgeThreat") || "脅威"}</span>
                        {/if}
                        {#if sc.ComplianceViolated}
                          <span class="px-1.5 py-0.5 rounded text-[10px] font-bold bg-purple-500/15 text-purple-700 dark:text-purple-300 border border-purple-500/30">{$_("report.twagentBadgeCompViolation") || "違反"}</span>
                        {/if}
                        {#if !sc.IsThreat && !sc.ComplianceViolated}
                          <span class="px-1.5 py-0.5 rounded text-[10px] font-bold bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border border-emerald-500/30">OK</span>
                        {/if}
                      </div>
                    {:else}
                      <span class="text-slate-400 text-xs">-</span>
                    {/if}
                  </td>
                  <td class="py-1 px-2 font-sans whitespace-nowrap">
                    <div class="flex flex-wrap items-center gap-1">
                      {#each getProtocols(sc) as proto}
                        {@const pLower = proto.toLowerCase()}
                        <span class="px-1.5 py-0.5 rounded text-[10px] font-bold uppercase
                          {pLower === 'mqtt' ? 'bg-cyan-500/15 text-cyan-700 dark:text-cyan-300 border border-cyan-500/30' :
                           pLower === 'syslog' ? 'bg-blue-500/15 text-blue-700 dark:text-blue-300 border border-blue-500/30' :
                           pLower === 'trap' ? 'bg-purple-500/15 text-purple-700 dark:text-purple-300 border border-purple-500/30' :
                           'bg-slate-500/15 text-slate-700 dark:text-slate-300 border border-slate-500/30'}">
                          {proto}
                        </span>
                      {/each}
                    </div>
                  </td>
                  <td class="py-1 px-2 text-right font-bold text-slate-700 dark:text-slate-300">{sc.Count.toLocaleString()}</td>
                  <td class="py-1 px-2 text-slate-500 whitespace-nowrap">{formatTimeStr(sc.LastTime)}</td>
                </tr>

                {#if isExpanded}
                  <tr class="bg-cyan-50/20 dark:bg-cyan-950/10 border-b border-cyan-100 dark:border-cyan-900/30">
                    <td colspan="9" class="p-3 text-xs space-y-2.5">
                      <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                        <div class="p-2.5 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-2">
                          <div class="flex items-center justify-between">
                            <span class="font-bold text-slate-700 dark:text-slate-300">{$_("report.twagentColCategory") || "詳細説明"}</span>
                            {#if sc.Compliance && sc.Compliance.length > 0}
                              <div class="flex flex-wrap gap-1">
                                {#each sc.Compliance as c}
                                  <span class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20">{c}</span>
                                {/each}
                              </div>
                            {/if}
                          </div>
                          <p class="text-slate-600 dark:text-slate-400 text-xs leading-relaxed">{sc.Description || sc.Title}</p>
                          {#if sc.Proof}
                            <div class="space-y-1">
                              <span class="text-[10px] font-semibold text-slate-500">{$_("report.twagentColProof") || "検出証跡 / パーミッション"}:</span>
                              <pre class="p-2 rounded bg-slate-100 dark:bg-slate-950 text-[11px] font-mono text-slate-700 dark:text-slate-300 overflow-x-auto whitespace-pre-wrap">{sc.Proof}</pre>
                            </div>
                          {/if}
                          {#if sc.Remediation}
                            <div class="p-2 rounded-lg bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-900/40 text-amber-800 dark:text-amber-200 text-xs">
                              <span class="font-bold">{$_("report.twagentColRemediation") || "是正推奨手順"}:</span> {sc.Remediation}
                            </div>
                          {/if}
                        </div>

                        <div class="p-2.5 rounded-xl bg-gradient-to-br from-indigo-50/50 to-purple-50/50 dark:from-indigo-950/30 dark:to-purple-950/30 border border-indigo-200/60 dark:border-indigo-900/40 space-y-2">
                          <div class="flex items-center justify-between">
                            <div class="flex items-center gap-1.5 text-indigo-700 dark:text-indigo-300 font-bold">
                              <Sparkles class="w-3.5 h-3.5" />
                              <span>tensai Local AI Judge</span>
                            </div>
                            {#if sc.Confidence}
                              <span class="text-[10px] font-mono text-indigo-600 dark:text-indigo-400">Confidence: {(sc.Confidence * 100).toFixed(0)}%</span>
                            {/if}
                          </div>
                          {#if sc.SummaryJa}
                            <div class="text-xs text-slate-700 dark:text-slate-300">
                              <span class="font-semibold text-indigo-600 dark:text-indigo-400">要約: </span>
                              {sc.SummaryJa}
                            </div>
                          {/if}
                          {#if sc.ActionJa}
                            <div class="p-2 rounded-lg bg-white/80 dark:bg-slate-900/80 border border-indigo-100 dark:border-indigo-900/40 text-xs text-slate-800 dark:text-slate-200">
                              <span class="font-bold text-indigo-700 dark:text-indigo-300">推奨対処: </span>
                              {sc.ActionJa}
                            </div>
                          {/if}
                          {#if !sc.SummaryJa && !sc.ActionJa}
                            <p class="text-slate-500 italic text-xs">ローカルLLM判定による助言なし (未判定または低深刻度)</p>
                          {/if}
                          <div class="pt-1 flex items-center justify-end gap-2">
                            <button
                              type="button"
                              onclick={(e) => {
                                e.stopPropagation();
                                showAiExplainModal = true;
                              }}
                              class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-semibold bg-indigo-600 text-white hover:bg-indigo-700 transition cursor-pointer"
                            >
                              <Sparkles class="w-3 h-3" />
                              <span>AIで解説</span>
                            </button>
                          </div>
                        </div>
                      </div>
                    </td>
                  </tr>
                {/if}

              {:else if activeTab === "tcp"}
                {@const t = item as TwAgentTCPEnt}
                <tr class="hover:bg-cyan-50/40 dark:hover:bg-cyan-950/20 transition-colors">
                  <td class="py-1 px-2 text-center text-slate-400">-</td>
                  <td class="py-1 px-2 text-right whitespace-nowrap">
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold {(t.Score ?? 50) >= 50 ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300' : (t.Score ?? 50) >= 35 ? 'bg-amber-500/15 text-amber-700 dark:text-amber-300' : 'bg-rose-500/15 text-rose-700 dark:text-rose-300'}">
                      {(t.Score ?? 50).toFixed(1)}
                    </span>
                  </td>
                  <td class="py-1 px-2 text-slate-700 dark:text-slate-300">{t.Host}</td>
                  <td class="py-1 px-2 font-bold text-slate-800 dark:text-slate-100">{t.Process || "-"}</td>
                  <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400">{t.RemoteIP}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400">{t.RemoteLoc || "-"}</td>
                  <td class="py-1 px-2">
                    <div class="flex flex-wrap gap-1">
                      {#each t.Services || [] as svc}
                        <span class="px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 text-[10px]">
                          {svc}
                        </span>
                      {/each}
                    </div>
                  </td>
                  <td class="py-1 px-2 text-right">
                    <span class="font-bold text-slate-800 dark:text-slate-200">{t.ConnCount}</span>
                    <span class="text-slate-400 text-[10px] ml-1">({(t.Count || 0).toLocaleString()})</span>
                  </td>
                  <td class="py-1 px-2 text-slate-500 whitespace-nowrap">{formatTimeStr(t.LastTime)}</td>
                </tr>
              {/if}
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    <ReportPagination
      bind:pageSize
      bind:currentPage
      totalCount={currentList.length}
    />
  </div>
</div>

<NodeDialog
  bind:show={showNodeModal}
  node={nodeToEdit}
  onSave={() => {
    onRefresh();
  }}
/>

<PollingDialog
  bind:show={showPollingModal}
  bind:polling={pollingToEdit}
  {nodes}
  onSave={() => {
    onRefresh();
  }}
/>

{#if showAiExplainModal}
  <ReportAIExplainModal
    bind:show={showAiExplainModal}
    reportType="twagent"
    title={$_("report.aiTwagentTitle") || "twagentneo エージェント監視 & AIセキュリティ総合診断"}
    summary={isJaLocale
      ? `監視エージェント数: ${kpis.totalAgents}台 (オンライン: ${kpis.onlineAgents}台, マップ未登録: ${kpis.unregisteredAgents}台), AI脅威判定: ${kpis.realThreats}件, AI誤検知除外: ${kpis.benignFiltered}件, コンプライアンス違反: ${kpis.compViolations}件, FIM変更イベント: ${kpis.totalFim}件, プロセス外部通信: ${kpis.totalTcp}件`
      : `Agents: ${kpis.totalAgents} (Online: ${kpis.onlineAgents}, Unregistered: ${kpis.unregisteredAgents}), AI Threats: ${kpis.realThreats}, AI Filtered Benign: ${kpis.benignFiltered}, Compliance Violations: ${kpis.compViolations}, FIM Events: ${kpis.totalFim}, Process TCP Flows: ${kpis.totalTcp}`}
    items={[
      ...agents.slice(0, 15).map((a) =>
        isJaLocale
          ? `[AGENT] ${a.Hostname || a.AgentID} (${a.IP}, ${a.OS} ${a.Version}): State=${a.Status}, CPU=${a.CPUPercent.toFixed(1)}%, Mem=${a.MemPercent.toFixed(1)}%, Disk=${a.DiskUsedPercent.toFixed(1)}%, Threats=${a.ThreatCount}, FIM=${a.FIMCount}`
          : `[AGENT] ${a.Hostname || a.AgentID} (${a.IP}, ${a.OS} ${a.Version}): State=${a.Status}, CPU=${a.CPUPercent.toFixed(1)}%, Mem=${a.MemPercent.toFixed(1)}%, Disk=${a.DiskUsedPercent.toFixed(1)}%, Threats=${a.ThreatCount}, FIM=${a.FIMCount}`
      ),
      ...alerts.filter((al) => al.IsThreat || al.ComplianceViolated).slice(0, 15).map((al) =>
        isJaLocale
          ? `[ALERT:${al.Level}] Host=${al.Host}, Rule=${al.RuleTitle}, Threat=${al.IsThreat} (${(al.Confidence * 100).toFixed(0)}%), Compliance=${(al.ComplianceTypes || []).join("|")}, Summary=${al.SummaryJa}`
          : `[ALERT:${al.Level}] Host=${al.Host}, Rule=${al.RuleTitle}, Threat=${al.IsThreat} (${(al.Confidence * 100).toFixed(0)}%), Compliance=${(al.ComplianceTypes || []).join("|")}, Summary=${al.SummaryJa}`
      ),
      ...fimEvents.slice(0, 10).map((f) => `[FIM:${f.Op}] Host=${f.Host}, Path=${f.Path}, Count=${f.Count}`),
      ...tcpConns.slice(0, 10).map((t) => `[TCP] Host=${t.Host}, Proc=${t.Process} -> ${t.RemoteIP} (${t.RemoteLoc}), Score=${(t.Score ?? 50).toFixed(1)}`),
    ]}
  />
{/if}
