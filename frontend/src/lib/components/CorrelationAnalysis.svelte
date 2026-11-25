<script lang="ts">
  import { onDestroy, onMount, tick } from "svelte";
  import { _, locale } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    GitCompareArrows,
    Search,
    ArrowLeftRight,
    Sparkles,
    Network,
    Play,
    X,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    BarChart3,
    TrendingUp,
    Activity,
    Grid3x3,
    AlertCircle,
    Info,
  } from "@lucide/svelte";
  import {
    getCorrelationCandidates,
    analyzePollingCorrelation,
    discoverPollingCorrelations,
    type CorrelationCandidate,
    type CorrelationDiscoverItem,
    type CorrelationDiscoverRes,
    type CorrelationPairRes,
  } from "../api";
  import {
    showCorrelationOverlayChart,
    showCorrelationScatterChart,
    showCorrelationLagChart,
    showCorrelationMatrixChart,
    showCorrelationNetworkChart,
  } from "../charts/correlation";
  import ReportPagination from "../views/reports/components/ReportPagination.svelte";
  import ReportAIExplainModal from "./ReportAIExplainModal.svelte";

  let {
    initialTab = "discover",
    initialPollingID1 = "",
    initialPollingID2 = "",
    initialBasePollingID = "",
    searchQuery = "",
    compact = false,
  }: {
    initialTab?: "discover" | "pair";
    initialPollingID1?: string;
    initialPollingID2?: string;
    initialBasePollingID?: string;
    searchQuery?: string;
    compact?: boolean;
  } = $props();

  const isJaLocale = $derived($locale?.startsWith("ja") ?? true);

  let activeTab = $state<"discover" | "pair">("discover");
  let candidates = $state<CorrelationCandidate[]>([]);
  let loadingCandidates = $state(false);
  let errorMsg = $state("");

  // Shared time/algorithm controls
  let rangeHours = $state<number>(24);
  let bucketSec = $state<number>(0);
  let algorithm = $state<string>("pearson");
  let maxLag = $state<number>(12);
  let includeState = $state<boolean>(false);

  // Discover state
  let basePollingID = $state<string>("");
  let minAbsScore = $state<number>(0.5);
  let minSamples = $state<number>(10);
  let crossNodeOnly = $state<boolean>(false);
  let discovering = $state<boolean>(false);
  let discoverRes = $state<CorrelationDiscoverRes | null>(null);
  let discoverAbort: AbortController | null = null;
  let showNetwork = $state<boolean>(true);
  let networkDom = $state<HTMLDivElement | null>(null);
  let networkChart: echarts.ECharts | null = null;

  // Discover table sort & pagination
  let sortCol = $state<"absScore" | "score" | "pValue" | "lag" | "samples">("absScore");
  let sortAsc = $state<boolean>(false);
  let pageSize = $state<number>(25);
  let currentPage = $state<number>(1);

  // Pair state
  let nodeFilter1 = $state<string>("");
  let nodeFilter2 = $state<string>("");
  let pollingID1 = $state<string>("");
  let pollingID2 = $state<string>("");
  let vars1 = $state<string[]>([]);
  let vars2 = $state<string[]>([]);
  let analyzing = $state<boolean>(false);
  let pairRes = $state<CorrelationPairRes | null>(null);
  let pairAbort: AbortController | null = null;
  let pairChartMode = $state<"overlay" | "scatter" | "lag" | "matrix">("overlay");
  let overlayMode = $state<"dual" | "norm">("dual");
  let pairChartDom = $state<HTMLDivElement | null>(null);
  let pairChart: echarts.ECharts | null = null;

  // AI Explain Modal
  let showAiExplainModal = $state(false);

  const uniqueNodes = $derived.by(() => {
    const map = new Map<string, string>();
    for (const c of candidates) {
      if (c.NodeID && !map.has(c.NodeID)) {
        map.set(c.NodeID, c.NodeName || c.NodeID);
      }
    }
    return Array.from(map.entries())
      .map(([id, name]) => ({ id, name }))
      .sort((a, b) => a.name.localeCompare(b.name));
  });

  const filteredCandidates1 = $derived(
    nodeFilter1 ? candidates.filter((c) => c.NodeID === nodeFilter1) : candidates,
  );
  const filteredCandidates2 = $derived(
    nodeFilter2 ? candidates.filter((c) => c.NodeID === nodeFilter2) : candidates,
  );

  const selectedCandidate1 = $derived(
    candidates.find((c) => c.PollingID === pollingID1) || null,
  );
  const selectedCandidate2 = $derived(
    candidates.find((c) => c.PollingID === pollingID2) || null,
  );

  const availVars1 = $derived.by(() => {
    const base = selectedCandidate1?.Vars ? [...selectedCandidate1.Vars] : [];
    if (includeState && !base.includes("state")) base.push("state");
    return base;
  });
  const availVars2 = $derived.by(() => {
    const base = selectedCandidate2?.Vars ? [...selectedCandidate2.Vars] : [];
    if (includeState && !base.includes("state")) base.push("state");
    return base;
  });

  const filteredDiscoverItems = $derived.by(() => {
    const items = discoverRes?.Items || [];
    const q = searchQuery.trim().toLowerCase();
    const list = q
      ? items.filter(
          (i) =>
            i.NodeName1.toLowerCase().includes(q) ||
            i.PollingName1.toLowerCase().includes(q) ||
            i.NodeName2.toLowerCase().includes(q) ||
            i.PollingName2.toLowerCase().includes(q) ||
            i.BestVar1.toLowerCase().includes(q) ||
            i.BestVar2.toLowerCase().includes(q),
        )
      : [...items];

    list.sort((a, b) => {
      let diff = 0;
      if (sortCol === "absScore") diff = a.AbsScore - b.AbsScore;
      else if (sortCol === "score") diff = a.Score - b.Score;
      else if (sortCol === "pValue") diff = a.PValue - b.PValue;
      else if (sortCol === "lag") diff = Math.abs(a.BestLagSec) - Math.abs(b.BestLagSec);
      else if (sortCol === "samples") diff = a.Samples - b.Samples;
      return sortAsc ? diff : -diff;
    });
    return list;
  });

  const pagedDiscoverItems = $derived.by(() => {
    if (pageSize <= 0) return filteredDiscoverItems;
    const start = (currentPage - 1) * pageSize;
    return filteredDiscoverItems.slice(start, start + pageSize);
  });

  const toggleSort = (col: typeof sortCol) => {
    if (sortCol === col) {
      sortAsc = !sortAsc;
    } else {
      sortCol = col;
      sortAsc = col === "pValue";
    }
    currentPage = 1;
  };

  const getScoreBadgeClass = (absScore: number) => {
    if (absScore >= 0.8) {
      return "bg-rose-100 text-rose-800 dark:bg-rose-900/40 dark:text-rose-300 border-rose-200 dark:border-rose-800";
    }
    if (absScore >= 0.6) {
      return "bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300 border-amber-200 dark:border-amber-800";
    }
    if (absScore >= 0.4) {
      return "bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border-blue-200 dark:border-blue-800";
    }
    return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 border-slate-200 dark:border-slate-700";
  };

  const formatPValue = (p: number) => {
    if (!Number.isFinite(p)) return "-";
    if (p < 0.0001) return "< 0.0001";
    return p.toFixed(4);
  };

  const isMLAlgo = (algo: string) =>
    algo === "mlp" || algo === "lstm" || algo === "knn";

  const getActivePairScore = (res: CorrelationPairRes): { score: number; p: number } => {
    switch (algorithm) {
      case "spearman":
        return { score: res.Spearman, p: res.SpearmanP };
      case "kendall":
        return { score: res.Kendall, p: res.KendallP };
      case "crosscorr":
        return { score: res.CrossCorr, p: res.CrossCorrP };
      case "dcor":
        return { score: res.DistanceCorr, p: res.DistanceCorrP };
      case "mlp":
        return { score: res.MLP ?? 0, p: 1 };
      case "lstm":
        return { score: res.LSTM ?? 0, p: 1 };
      case "knn":
        return { score: res.KNN ?? 0, p: 1 };
      default:
        return { score: res.Pearson, p: res.PearsonP };
    }
  };

  const getInterpretationText = (res: CorrelationPairRes): string => {
    const { score, p } = getActivePairScore(res);
    const abs = Math.abs(score);
    let strength = $_("Correlation.strengthNone");
    if (abs >= 0.8) strength = $_("Correlation.strengthVeryStrong");
    else if (abs >= 0.6) strength = $_("Correlation.strengthStrong");
    else if (abs >= 0.4) strength = $_("Correlation.strengthModerate");
    else if (abs >= 0.2) strength = $_("Correlation.strengthWeak");

    let direction = "";
    if (algorithm === "dcor") {
      direction = $_("Correlation.directionNonlinear");
    } else if (algorithm === "mlp" || algorithm === "lstm") {
      direction = $_("Correlation.directionPredictive");
    } else if (algorithm === "knn") {
      direction = $_("Correlation.directionRegime");
    } else {
      direction =
        score >= 0
          ? $_("Correlation.directionPositive")
          : $_("Correlation.directionNegative");
    }

    if (isMLAlgo(algorithm)) {
      const mlExtra =
        algorithm === "knn" && res.RegimeResult
          ? ` (BalAcc=${(res.RegimeResult.BalancedAccuracy * 100).toFixed(1)}%)`
          : "";
      return isJaLocale
        ? `${strength}${direction}（tensai ML スコア=${score.toFixed(4)}${mlExtra}）`
        : `${strength} ${direction} (tensai ML score=${score.toFixed(4)}${mlExtra})`;
    }

    const sig =
      p < 0.05 ? $_("Correlation.significant") : $_("Correlation.notSignificant");

    let lagPart = "";
    if (algorithm === "crosscorr" || res.BestLagSeconds !== 0) {
      if (res.BestLagSeconds > 0) {
        lagPart = ` (${$_("Correlation.lagLead", { values: { sec: res.BestLagSeconds } })})`;
      } else if (res.BestLagSeconds < 0) {
        lagPart = ` (${$_("Correlation.lagBehind", { values: { sec: Math.abs(res.BestLagSeconds) } })})`;
      } else {
        lagPart = ` (${$_("Correlation.lagSync")})`;
      }
    }
    if (isJaLocale) {
      return `${strength}${direction}（${sig}、p=${formatPValue(p)}）${lagPart}`;
    }
    return `${strength} ${direction} (${sig}, p=${formatPValue(p)})${lagPart}`;
  };

  const renderNetwork = async () => {
    await tick();
    if (!networkDom || !discoverRes?.Items?.length || !showNetwork) {
      if (networkChart) {
        networkChart.dispose();
        networkChart = null;
      }
      return;
    }
    networkChart = showCorrelationNetworkChart(
      networkDom,
      discoverRes.Items,
      (item) => inspectDiscoveredPair(item),
      {
        lagLabel: $_("Correlation.colLag"),
      },
    );
  };

  const renderPairChart = async () => {
    await tick();
    if (!pairChartDom || !pairRes || pairRes.Error) {
      if (pairChart) {
        pairChart.dispose();
        pairChart = null;
      }
      return;
    }
    if (pairChartMode === "overlay") {
      pairChart = showCorrelationOverlayChart(pairChartDom, pairRes, overlayMode, {
        normAxisLabel: $_("Correlation.overlayNorm"),
      });
    } else if (pairChartMode === "scatter") {
      pairChart = showCorrelationScatterChart(pairChartDom, pairRes);
    } else if (pairChartMode === "lag") {
      pairChart = showCorrelationLagChart(pairChartDom, pairRes, {
        correlation: $_("Correlation.colScore"),
      });
    } else if (pairChartMode === "matrix") {
      pairChart = showCorrelationMatrixChart(pairChartDom, pairRes, (v1, v2) => {
        vars1 = [v1];
        vars2 = [v2];
        runPairAnalysis();
      });
    }
  };

  $effect(() => {
    if (activeTab === "discover" && showNetwork && discoverRes?.Items?.length && networkDom) {
      renderNetwork();
    }
  });

  $effect(() => {
    if (activeTab === "pair" && pairRes && !pairRes.Error && pairChartDom) {
      const _mode = pairChartMode;
      const _ov = overlayMode;
      const _res = pairRes;
      void _mode;
      void _ov;
      void _res;
      renderPairChart();
    }
  });

  const runDiscover = async () => {
    if (discovering) return;
    errorMsg = "";
    discovering = true;
    currentPage = 1;
    discoverAbort = new AbortController();
    const now = Math.floor(Date.now() / 1000);
    try {
      discoverRes = await discoverPollingCorrelations(
        {
          BasePollingID: basePollingID,
          StartTime: now - rangeHours * 3600,
          EndTime: now,
          BucketSec: bucketSec,
          Algorithm: algorithm,
          MaxLag: Math.min(30, maxLag),
          MinAbsScore: minAbsScore,
          MinSamples: minSamples,
          CrossNodeOnly: crossNodeOnly,
          IncludeState: includeState,
          Limit: 100,
        },
        discoverAbort.signal,
      );
      await renderNetwork();
    } catch (e: any) {
      if (e?.name === "AbortError") {
        errorMsg = $_("Correlation.canceledNotice");
      } else {
        errorMsg = e?.message || String(e);
      }
    } finally {
      discovering = false;
      discoverAbort = null;
    }
  };

  const cancelDiscover = () => {
    if (discoverAbort) {
      discoverAbort.abort();
    }
  };

  const runPairAnalysis = async () => {
    if (!pollingID1 || !pollingID2 || analyzing) return;
    errorMsg = "";
    analyzing = true;
    if (pairAbort) pairAbort.abort();
    pairAbort = new AbortController();
    const now = Math.floor(Date.now() / 1000);
    try {
      pairRes = await analyzePollingCorrelation(
        {
          PollingID1: pollingID1,
          PollingID2: pollingID2,
          Vars1: vars1,
          Vars2: vars2,
          StartTime: now - rangeHours * 3600,
          EndTime: now,
          BucketSec: bucketSec,
          Algorithm: algorithm,
          MaxLag: maxLag,
          IncludeState: includeState,
        },
        pairAbort.signal,
      );
      if (pairRes.Error) {
        errorMsg = pairRes.Error;
      } else {
        await renderPairChart();
      }
    } catch (e: any) {
      if (e?.name !== "AbortError") {
        errorMsg = e?.message || String(e);
      }
    } finally {
      analyzing = false;
      pairAbort = null;
    }
  };

  const inspectDiscoveredPair = (item: CorrelationDiscoverItem) => {
    pollingID1 = item.PollingID1;
    pollingID2 = item.PollingID2;
    vars1 = item.BestVar1 ? item.BestVar1.split(",").filter(Boolean) : [];
    vars2 = item.BestVar2 ? item.BestVar2.split(",").filter(Boolean) : [];
    activeTab = "pair";
    if (algorithm === "crosscorr" || item.BestLagSec !== 0) {
      pairChartMode = "overlay";
    }
    runPairAnalysis();
  };

  const swapPair = () => {
    const tmpId = pollingID1;
    pollingID1 = pollingID2;
    pollingID2 = tmpId;
    const tmpNode = nodeFilter1;
    nodeFilter1 = nodeFilter2;
    nodeFilter2 = tmpNode;
    const tmpVars = vars1;
    vars1 = vars2;
    vars2 = tmpVars;
    if (pollingID1 && pollingID2) {
      runPairAnalysis();
    }
  };

  const toggleVar = (side: 1 | 2, v: string) => {
    if (side === 1) {
      vars1 = vars1.includes(v) ? vars1.filter((x) => x !== v) : [...vars1, v];
    } else {
      vars2 = vars2.includes(v) ? vars2.filter((x) => x !== v) : [...vars2, v];
    }
  };

  const loadCandidates = async () => {
    loadingCandidates = true;
    errorMsg = "";
    try {
      candidates = await getCorrelationCandidates();
    } catch (e: any) {
      errorMsg = e?.message || String(e);
    } finally {
      loadingCandidates = false;
    }
  };

  const handleResize = () => {
    networkChart?.resize();
    pairChart?.resize();
  };

  onMount(async () => {
    activeTab = initialTab;
    basePollingID = initialBasePollingID;
    pollingID1 = initialPollingID1;
    pollingID2 = initialPollingID2;
    window.addEventListener("resize", handleResize);
    await loadCandidates();
    if (activeTab === "pair" && pollingID1 && pollingID2) {
      await runPairAnalysis();
    } else if (activeTab === "discover" && basePollingID) {
      await runDiscover();
    }
  });

  onDestroy(() => {
    window.removeEventListener("resize", handleResize);
    discoverAbort?.abort();
    pairAbort?.abort();
    networkChart?.dispose();
    pairChart?.dispose();
  });

  // AI Explain payload
  const aiSummary = $derived.by(() => {
    if (activeTab === "pair" && pairRes && !pairRes.Error) {
      const { score, p } = getActivePairScore(pairRes);
      return isJaLocale
        ? `ペア比較: ${pairRes.NodeName1}/${pairRes.PollingName1}(${pairRes.BestVar1}) vs ${pairRes.NodeName2}/${pairRes.PollingName2}(${pairRes.BestVar2}) | アルゴリズム: ${algorithm}, スコア: ${score.toFixed(4)}, p値: ${formatPValue(p)}, ラグ: ${pairRes.BestLagSeconds}秒, サンプル数: ${pairRes.SampleCount}`
        : `Pair Comparison: ${pairRes.NodeName1}/${pairRes.PollingName1}(${pairRes.BestVar1}) vs ${pairRes.NodeName2}/${pairRes.PollingName2}(${pairRes.BestVar2}) | Algorithm: ${algorithm}, Score: ${score.toFixed(4)}, p=${formatPValue(p)}, Lag: ${pairRes.BestLagSeconds}s, Samples: ${pairRes.SampleCount}`;
    }
    if (discoverRes) {
      return isJaLocale
        ? `相関探索結果: 対象ポーリング ${discoverRes.TotalPollings}件, 評価ペア ${discoverRes.PairsEvaluated}件, 検出 ${discoverRes.Items?.length || 0}件 (アルゴリズム: ${discoverRes.Algorithm}, バケット: ${discoverRes.BucketSec}秒)`
        : `Correlation Discovery: Pollings ${discoverRes.TotalPollings}, Evaluated ${discoverRes.PairsEvaluated}, Found ${discoverRes.Items?.length || 0} (Algorithm: ${discoverRes.Algorithm}, Bucket: ${discoverRes.BucketSec}s)`;
    }
    return "";
  });

  const aiItems = $derived.by(() => {
    if (activeTab === "pair" && pairRes && !pairRes.Error) {
      return [
        `Pearson r=${pairRes.Pearson.toFixed(4)} (p=${formatPValue(pairRes.PearsonP)})`,
        `Spearman rho=${pairRes.Spearman.toFixed(4)} (p=${formatPValue(pairRes.SpearmanP)})`,
        `Kendall tau=${pairRes.Kendall.toFixed(4)} (p=${formatPValue(pairRes.KendallP)})`,
        `CrossCorr=${pairRes.CrossCorr.toFixed(4)} (p=${formatPValue(pairRes.CrossCorrP)}, BestLag=${pairRes.BestLagSeconds}s)`,
        `DistanceCorr=${pairRes.DistanceCorr.toFixed(4)} (p=${formatPValue(pairRes.DistanceCorrP)})`,
        `Neural MLP=${(pairRes.MLP ?? 0).toFixed(4)}, LSTM=${(pairRes.LSTM ?? 0).toFixed(4)}, kNN Regime=${(pairRes.KNN ?? 0).toFixed(4)}${pairRes.RegimeResult ? ` (BalAcc=${(pairRes.RegimeResult.BalancedAccuracy * 100).toFixed(1)}%)` : ""}`,
        ...(pairRes.FeatureImportances || []).slice(0, 6).map(
          (fi) => `MLP Feature Importance ${fi.Feature}: ${(fi.Importance * 100).toFixed(1)}%`,
        ),
        ...(pairRes.VarMatrix || []).slice(0, 15).map(
          (m) => `Matrix ${m.Var1} x ${m.Var2}: score=${m.Score.toFixed(4)}, p=${formatPValue(m.PValue)}`,
        ),
      ];
    }
    return (discoverRes?.Items || []).slice(0, 25).map(
      (it) =>
        `${it.NodeName1}/${it.PollingName1}(${it.BestVar1}) <-> ${it.NodeName2}/${it.PollingName2}(${it.BestVar2}): score=${it.Score.toFixed(4)}, p=${formatPValue(it.PValue)}, lag=${it.BestLagSec}s, samples=${it.Samples}`,
    );
  });
</script>

<div class="flex flex-col gap-4 {compact ? '' : 'p-1'}">
  <!-- Top Tab Switcher & Global Controls -->
  <div
    class="flex flex-wrap items-center justify-between gap-3 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-xs"
  >
    <div class="flex items-center gap-1.5 bg-slate-100 dark:bg-slate-800 p-1 rounded-lg">
      <button
        type="button"
        onclick={() => {
          activeTab = "discover";
          if (algorithm === "lstm") algorithm = "pearson";
        }}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors {activeTab ===
        'discover'
          ? 'bg-white dark:bg-slate-700 text-blue-600 dark:text-blue-400 shadow-xs'
          : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
      >
        <Search class="w-3.5 h-3.5" />
        {$_("Correlation.tabDiscover")}
      </button>
      <button
        type="button"
        onclick={() => (activeTab = "pair")}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors {activeTab ===
        'pair'
          ? 'bg-white dark:bg-slate-700 text-blue-600 dark:text-blue-400 shadow-xs'
          : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
      >
        <GitCompareArrows class="w-3.5 h-3.5" />
        {$_("Correlation.tabPair")}
      </button>
    </div>

    <!-- Shared Range / Bucket / Algorithm -->
    <div class="flex flex-wrap items-center gap-2 text-xs">
      <label class="flex items-center gap-1.5 text-slate-600 dark:text-slate-400">
        <span>{$_("Correlation.range")}:</span>
        <select
          bind:value={rangeHours}
          class="bg-slate-50 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-md px-2 py-1 text-slate-800 dark:text-slate-200"
        >
          <option value={6}>{$_("Correlation.range6h")}</option>
          <option value={24}>{$_("Correlation.range24h")}</option>
          <option value={72}>{$_("Correlation.range3d")}</option>
          <option value={168}>{$_("Correlation.range7d")}</option>
          <option value={336}>{$_("Correlation.range14d")}</option>
          <option value={720}>{$_("Correlation.range30d")}</option>
        </select>
      </label>

      <label class="flex items-center gap-1.5 text-slate-600 dark:text-slate-400">
        <span>{$_("Correlation.bucket")}:</span>
        <select
          bind:value={bucketSec}
          class="bg-slate-50 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-md px-2 py-1 text-slate-800 dark:text-slate-200"
        >
          <option value={0}>{$_("Correlation.bucketAuto")}</option>
          <option value={60}>60s</option>
          <option value={300}>5m</option>
          <option value={900}>15m</option>
          <option value={3600}>1h</option>
        </select>
      </label>

      <label class="flex items-center gap-1.5 text-slate-600 dark:text-slate-400">
        <span>{$_("Correlation.algorithm")}:</span>
        <select
          bind:value={algorithm}
          onchange={() => {
            if (isMLAlgo(algorithm) && minAbsScore >= 0.5) {
              minAbsScore = 0.2;
            }
          }}
          class="bg-slate-50 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-md px-2 py-1 text-slate-800 dark:text-slate-200"
        >
          <option value="pearson">{$_("Correlation.algoPearson")}</option>
          <option value="spearman">{$_("Correlation.algoSpearman")}</option>
          <option value="kendall">{$_("Correlation.algoKendall")}</option>
          <option value="crosscorr">{$_("Correlation.algoCrossCorr")}</option>
          <option value="dcor">{$_("Correlation.algoDistance")}</option>
          <option value="mlp">{$_("Correlation.algoMLP")}</option>
          <option value="knn">{$_("Correlation.algoKNN")}</option>
          {#if activeTab === "pair"}
            <option value="lstm">{$_("Correlation.algoLSTM")}</option>
          {/if}
        </select>
      </label>

      <label class="flex items-center gap-1.5 text-slate-600 dark:text-slate-400 cursor-pointer">
        <input
          type="checkbox"
          bind:checked={includeState}
          class="rounded border-slate-300 dark:border-slate-600 text-blue-600"
        />
        <span>{$_("Correlation.includeState")}</span>
      </label>

      {#if (activeTab === "discover" && discoverRes?.Items?.length) || (activeTab === "pair" && pairRes && !pairRes.Error)}
        <button
          type="button"
          onclick={() => (showAiExplainModal = true)}
          class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-purple-600 hover:bg-purple-700 text-white font-medium transition-colors"
        >
          <Sparkles class="w-3.5 h-3.5" />
          {$_("Correlation.aiExplain")}
        </button>
      {/if}
    </div>
  </div>

  {#if errorMsg}
    <div
      class="flex items-center gap-2 p-3 rounded-lg bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-800 text-rose-700 dark:text-rose-300 text-xs"
    >
      <AlertCircle class="w-4 h-4 shrink-0" />
      <span>{errorMsg}</span>
    </div>
  {/if}

  {#if !loadingCandidates && candidates.length === 0}
    <div
      class="flex flex-col items-center justify-center p-10 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 text-slate-500 dark:text-slate-400 text-sm text-center"
    >
      <Info class="w-8 h-8 mb-2 opacity-50" />
      <p>{$_("Correlation.noCandidates")}</p>
    </div>
  {:else if activeTab === "discover"}
    <!-- DISCOVER TAB -->
    <div
      class="flex flex-wrap items-end gap-3 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-xs text-xs"
    >
      <label class="flex flex-col gap-1 text-slate-600 dark:text-slate-400 min-w-[220px] flex-1">
        <span>{$_("Correlation.basePolling")}</span>
        <select
          bind:value={basePollingID}
          class="bg-slate-50 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-md px-2.5 py-1.5 text-slate-800 dark:text-slate-200"
        >
          <option value="">{$_("Correlation.allPairs")}</option>
          {#each candidates as c}
            <option value={c.PollingID}>
              {c.NodeName} / {c.PollingName} ({c.Type})
            </option>
          {/each}
        </select>
      </label>

      <label class="flex flex-col gap-1 text-slate-600 dark:text-slate-400 w-44">
        <span>{$_("Correlation.minScore")}</span>
        <select
          bind:value={minAbsScore}
          class="bg-slate-50 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-md px-2.5 py-1.5 text-slate-800 dark:text-slate-200"
        >
          <option value={0}>{$_("Correlation.minScoreAll")}</option>
          <option value={0.2}>&gt;= 0.2</option>
          <option value={0.4}>&gt;= 0.4</option>
          <option value={0.5}>&gt;= 0.5</option>
          <option value={0.6}>&gt;= 0.6</option>
          <option value={0.7}>&gt;= 0.7</option>
          <option value={0.8}>&gt;= 0.8</option>
        </select>
      </label>

      <label class="flex flex-col gap-1 text-slate-600 dark:text-slate-400 w-28">
        <span>{$_("Correlation.minSamples")}</span>
        <input
          type="number"
          min="3"
          max="1000"
          bind:value={minSamples}
          class="bg-slate-50 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-md px-2.5 py-1.5 text-slate-800 dark:text-slate-200"
        />
      </label>

      <label class="flex items-center gap-1.5 pb-1.5 text-slate-600 dark:text-slate-400 cursor-pointer">
        <input
          type="checkbox"
          bind:checked={crossNodeOnly}
          class="rounded border-slate-300 dark:border-slate-600 text-blue-600"
        />
        <span>{$_("Correlation.crossNodeOnly")}</span>
      </label>

      <div class="flex items-center gap-2 ml-auto">
        {#if discoverRes?.Items?.length}
          <button
            type="button"
            onclick={() => (showNetwork = !showNetwork)}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-slate-300 dark:border-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <Network class="w-3.5 h-3.5" />
            {$_("Correlation.showNetwork")}
          </button>
        {/if}

        {#if discovering}
          <button
            type="button"
            onclick={cancelDiscover}
            class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-md bg-rose-600 hover:bg-rose-700 text-white font-medium transition-colors"
          >
            <X class="w-3.5 h-3.5" />
            {$_("Correlation.cancel")}
          </button>
        {:else}
          <button
            type="button"
            onclick={runDiscover}
            class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-md bg-blue-600 hover:bg-blue-700 text-white font-medium transition-colors"
          >
            <Play class="w-3.5 h-3.5" />
            {$_("Correlation.discoverBtn")}
          </button>
        {/if}
      </div>
    </div>

    {#if discovering}
      <div
        class="flex items-center justify-center gap-2 p-8 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl text-sm text-slate-600 dark:text-slate-300"
      >
        <span class="w-4 h-4 border-2 border-blue-500 border-t-transparent rounded-full animate-spin"></span>
        <span>{$_("Correlation.discovering")}</span>
      </div>
    {:else if discoverRes}
      <div class="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400 px-1">
        <span>
          {$_("Correlation.statsSummary", {
            values: {
              pollings: discoverRes.TotalPollings,
              pairs: discoverRes.PairsEvaluated,
              found: discoverRes.Items?.length || 0,
            },
          })}
        </span>
        <span>
          {$_("Correlation.bucketSize")}: {discoverRes.BucketSec}s
        </span>
      </div>

      {#if showNetwork && (discoverRes.Items?.length || 0) > 0}
        <div
          class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-xs"
        >
          <div class="text-xs font-semibold text-slate-700 dark:text-slate-300 mb-2 flex items-center gap-1.5">
            <Network class="w-4 h-4 text-blue-500" />
            {$_("Correlation.networkTitle")}
          </div>
          <div bind:this={networkDom} class="w-full h-80"></div>
        </div>
      {/if}

      <div
        class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl shadow-xs overflow-hidden"
      >
        {#if filteredDiscoverItems.length === 0}
          <div class="p-8 text-center text-sm text-slate-500 dark:text-slate-400">
            {$_("Correlation.noResults")}
          </div>
        {:else}
          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead
                class="bg-slate-50 dark:bg-slate-800/70 text-slate-600 dark:text-slate-300 border-b border-slate-200 dark:border-slate-700"
              >
                <tr>
                  <th class="px-3 py-2.5 font-semibold">{$_("Correlation.colNodePolling1")}</th>
                  <th class="px-3 py-2.5 font-semibold">{$_("Correlation.colNodePolling2")}</th>
                  <th class="px-3 py-2.5 font-semibold">{$_("Correlation.colVars")}</th>
                  <th class="px-3 py-2.5 font-semibold">
                    <button
                      type="button"
                      onclick={() => toggleSort("score")}
                      class="inline-flex items-center gap-1 hover:text-blue-600 dark:hover:text-blue-400"
                    >
                      {$_("Correlation.colScore")}
                      {#if sortCol === "score"}
                        {#if sortAsc}<ArrowUp class="w-3 h-3" />{:else}<ArrowDown class="w-3 h-3" />{/if}
                      {:else}
                        <ArrowUpDown class="w-3 h-3 opacity-40" />
                      {/if}
                    </button>
                  </th>
                  <th class="px-3 py-2.5 font-semibold">
                    <button
                      type="button"
                      onclick={() => toggleSort("pValue")}
                      class="inline-flex items-center gap-1 hover:text-blue-600 dark:hover:text-blue-400"
                    >
                      {$_("Correlation.colPValue")}
                      {#if sortCol === "pValue"}
                        {#if sortAsc}<ArrowUp class="w-3 h-3" />{:else}<ArrowDown class="w-3 h-3" />{/if}
                      {:else}
                        <ArrowUpDown class="w-3 h-3 opacity-40" />
                      {/if}
                    </button>
                  </th>
                  <th class="px-3 py-2.5 font-semibold">
                    <button
                      type="button"
                      onclick={() => toggleSort("lag")}
                      class="inline-flex items-center gap-1 hover:text-blue-600 dark:hover:text-blue-400"
                    >
                      {$_("Correlation.colLag")}
                      {#if sortCol === "lag"}
                        {#if sortAsc}<ArrowUp class="w-3 h-3" />{:else}<ArrowDown class="w-3 h-3" />{/if}
                      {:else}
                        <ArrowUpDown class="w-3 h-3 opacity-40" />
                      {/if}
                    </button>
                  </th>
                  <th class="px-3 py-2.5 font-semibold">
                    <button
                      type="button"
                      onclick={() => toggleSort("samples")}
                      class="inline-flex items-center gap-1 hover:text-blue-600 dark:hover:text-blue-400"
                    >
                      {$_("Correlation.colSamples")}
                      {#if sortCol === "samples"}
                        {#if sortAsc}<ArrowUp class="w-3 h-3" />{:else}<ArrowDown class="w-3 h-3" />{/if}
                      {:else}
                        <ArrowUpDown class="w-3 h-3 opacity-40" />
                      {/if}
                    </button>
                  </th>
                  <th class="px-3 py-2.5 font-semibold text-right">{$_("Correlation.colActions")}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
                {#each pagedDiscoverItems as item}
                  <tr
                    class="hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors cursor-pointer"
                    onclick={() => inspectDiscoveredPair(item)}
                  >
                    <td class="px-3 py-2">
                      <div class="font-medium text-slate-800 dark:text-slate-200">{item.NodeName1}</div>
                      <div class="text-slate-500 dark:text-slate-400">{item.PollingName1}</div>
                    </td>
                    <td class="px-3 py-2">
                      <div class="font-medium text-slate-800 dark:text-slate-200">{item.NodeName2}</div>
                      <div class="text-slate-500 dark:text-slate-400">{item.PollingName2}</div>
                    </td>
                    <td class="px-3 py-2 font-mono text-[11px] text-slate-600 dark:text-slate-300">
                      {item.BestVar1} ↔ {item.BestVar2}
                    </td>
                    <td class="px-3 py-2">
                      <span
                        class="inline-flex items-center px-2 py-0.5 rounded border font-mono font-semibold {getScoreBadgeClass(
                          item.AbsScore,
                        )}"
                      >
                        {item.Score.toFixed(4)}
                      </span>
                    </td>
                    <td class="px-3 py-2 font-mono text-slate-600 dark:text-slate-400">
                      {formatPValue(item.PValue)}
                    </td>
                    <td class="px-3 py-2 font-mono text-slate-600 dark:text-slate-400">
                      {item.BestLagSec !== 0 ? `${item.BestLagSec > 0 ? "+" : ""}${item.BestLagSec}s` : "0s"}
                    </td>
                    <td class="px-3 py-2 font-mono text-slate-600 dark:text-slate-400">
                      {item.Samples}
                    </td>
                    <td class="px-3 py-2 text-right">
                      <button
                        type="button"
                        onclick={(e) => {
                          e.stopPropagation();
                          inspectDiscoveredPair(item);
                        }}
                        class="px-2.5 py-1 rounded bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/50 font-medium"
                      >
                        {$_("Correlation.detailBtn")}
                      </button>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
          <ReportPagination
            bind:pageSize
            bind:currentPage
            totalCount={filteredDiscoverItems.length}
          />
        {/if}
      </div>
    {/if}
  {:else}
    <!-- PAIR COMPARISON TAB -->
    <div
      class="grid grid-cols-1 lg:grid-cols-11 gap-3 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-xs text-xs"
    >
      <!-- Polling A -->
      <div class="lg:col-span-5 flex flex-col gap-2">
        <div class="font-semibold text-blue-600 dark:text-blue-400">{$_("Correlation.pollingA")}</div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <select
            bind:value={nodeFilter1}
            class="bg-slate-50 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-md px-2 py-1.5 text-slate-800 dark:text-slate-200"
          >
            <option value="">{$_("Correlation.filterNode")}</option>
            {#each uniqueNodes as n}
              <option value={n.id}>{n.name}</option>
            {/each}
          </select>
          <select
            bind:value={pollingID1}
            onchange={() => (vars1 = [])}
            class="bg-slate-50 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-md px-2 py-1.5 text-slate-800 dark:text-slate-200"
          >
            <option value="">{$_("Correlation.selectPolling")}</option>
            {#each filteredCandidates1 as c}
              <option value={c.PollingID}>{c.NodeName} / {c.PollingName}</option>
            {/each}
          </select>
        </div>
        {#if availVars1.length > 0}
          <div class="flex flex-wrap gap-1 items-center">
            <span class="text-slate-500 dark:text-slate-400 mr-1">{$_("Correlation.variables")}:</span>
            {#each availVars1 as v}
              <button
                type="button"
                onclick={() => toggleVar(1, v)}
                class="px-2 py-0.5 rounded-full border text-[11px] transition-colors {vars1.includes(
                  v,
                )
                  ? 'bg-blue-600 border-blue-600 text-white'
                  : 'bg-slate-50 dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-slate-600 dark:text-slate-300'}"
              >
                {v}
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <!-- Swap & Analyze -->
      <div class="lg:col-span-1 flex lg:flex-col items-center justify-center gap-2">
        <button
          type="button"
          onclick={swapPair}
          title={$_("Correlation.swap")}
          class="p-2 rounded-lg border border-slate-300 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <ArrowLeftRight class="w-4 h-4" />
        </button>
        <button
          type="button"
          disabled={!pollingID1 || !pollingID2 || analyzing}
          onclick={runPairAnalysis}
          class="px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white font-medium flex items-center gap-1 whitespace-nowrap"
        >
          <Play class="w-3.5 h-3.5" />
          {analyzing ? $_("Correlation.analyzing") : $_("Correlation.analyzeBtn")}
        </button>
      </div>

      <!-- Polling B -->
      <div class="lg:col-span-5 flex flex-col gap-2">
        <div class="font-semibold text-orange-600 dark:text-orange-400">{$_("Correlation.pollingB")}</div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <select
            bind:value={nodeFilter2}
            class="bg-slate-50 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-md px-2 py-1.5 text-slate-800 dark:text-slate-200"
          >
            <option value="">{$_("Correlation.filterNode")}</option>
            {#each uniqueNodes as n}
              <option value={n.id}>{n.name}</option>
            {/each}
          </select>
          <select
            bind:value={pollingID2}
            onchange={() => (vars2 = [])}
            class="bg-slate-50 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-md px-2 py-1.5 text-slate-800 dark:text-slate-200"
          >
            <option value="">{$_("Correlation.selectPolling")}</option>
            {#each filteredCandidates2 as c}
              <option value={c.PollingID}>{c.NodeName} / {c.PollingName}</option>
            {/each}
          </select>
        </div>
        {#if availVars2.length > 0}
          <div class="flex flex-wrap gap-1 items-center">
            <span class="text-slate-500 dark:text-slate-400 mr-1">{$_("Correlation.variables")}:</span>
            {#each availVars2 as v}
              <button
                type="button"
                onclick={() => toggleVar(2, v)}
                class="px-2 py-0.5 rounded-full border text-[11px] transition-colors {vars2.includes(
                  v,
                )
                  ? 'bg-orange-500 border-orange-500 text-white'
                  : 'bg-slate-50 dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-slate-600 dark:text-slate-300'}"
              >
                {v}
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </div>

    {#if pairRes && !pairRes.Error}
      <!-- Interpretation Banner -->
      <div
        class="flex flex-wrap items-center justify-between gap-2 p-3 rounded-xl bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800 text-xs text-blue-900 dark:text-blue-200"
      >
        <div class="font-medium">
          {getInterpretationText(pairRes)}
        </div>
        <div class="flex items-center gap-3 text-slate-600 dark:text-slate-400">
          <span>{$_("Correlation.bestVars")}: <b class="font-mono">{pairRes.BestVar1} ↔ {pairRes.BestVar2}</b></span>
          <span>{$_("Correlation.samples")}: <b>{pairRes.SampleCount}</b></span>
          <span>{$_("Correlation.bucketSize")}: <b>{pairRes.BucketSec}s</b></span>
        </div>
      </div>

      <!-- 8 Algorithm Score Cards (Statistical + tensai ML) -->
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-2.5">
        {#each [
          { key: "pearson", label: "Pearson r", val: pairRes.Pearson, p: pairRes.PearsonP, sub: "", desc: $_("Correlation.algoDescPearson"), ml: false },
          { key: "spearman", label: "Spearman ρ", val: pairRes.Spearman, p: pairRes.SpearmanP, sub: "", desc: $_("Correlation.algoDescSpearman"), ml: false },
          { key: "kendall", label: "Kendall τ", val: pairRes.Kendall, p: pairRes.KendallP, sub: "", desc: $_("Correlation.algoDescKendall"), ml: false },
          { key: "crosscorr", label: "Cross-Corr", val: pairRes.CrossCorr, p: pairRes.CrossCorrP, sub: `Lag: ${pairRes.BestLagSeconds}s`, desc: $_("Correlation.algoDescCrossCorr"), ml: false },
          { key: "dcor", label: "Distance Corr", val: pairRes.DistanceCorr, p: pairRes.DistanceCorrP, sub: "", desc: $_("Correlation.algoDescDCor"), ml: false },
          { key: "mlp", label: "Neural MLP", val: pairRes.MLP ?? 0, p: 1, sub: "", desc: $_("Correlation.algoDescMLP"), ml: true },
          { key: "lstm", label: "LSTM (RNN)", val: pairRes.LSTM ?? 0, p: 1, sub: "", desc: $_("Correlation.algoDescLSTM"), ml: true },
          { key: "knn", label: "kNN Regime", val: pairRes.KNN ?? 0, p: 1, sub: pairRes.RegimeResult ? `BalAcc: ${(pairRes.RegimeResult.BalancedAccuracy * 100).toFixed(1)}%` : "", desc: $_("Correlation.algoDescKNN"), ml: true },
        ] as card}
          <button
            type="button"
            onclick={() => {
              algorithm = card.key;
              if (card.key === "crosscorr") pairChartMode = "lag";
            }}
            class="text-left p-3 rounded-xl border transition-all {algorithm === card.key
              ? 'bg-blue-50/70 dark:bg-blue-950/40 border-blue-400 dark:border-blue-600 ring-1 ring-blue-400'
              : 'bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 hover:border-slate-300 dark:hover:border-slate-700'}"
          >
            <div class="flex items-center justify-between gap-1">
              <span class="text-[11px] font-medium text-slate-500 dark:text-slate-400">{card.label}</span>
              {#if card.ml}
                <span class="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300">
                  tensai ML
                </span>
              {/if}
            </div>
            <div class="text-lg font-bold font-mono text-slate-900 dark:text-slate-100 mt-0.5">
              {card.val.toFixed(4)}
            </div>
            <div class="text-[11px] text-slate-500 dark:text-slate-400 flex justify-between mt-0.5">
              <span>{card.ml ? "R / Score" : `p=${formatPValue(card.p)}`}</span>
              {#if card.sub}<span>{card.sub}</span>{/if}
            </div>
            <div class="text-[10px] text-slate-400 dark:text-slate-500 mt-1 leading-tight">
              {card.desc}
            </div>
          </button>
        {/each}
      </div>

      <!-- ML Insights (MLP Feature Importance & kNN Regime Confusion Matrix) -->
      {#if (pairRes.FeatureImportances && pairRes.FeatureImportances.length > 0) || pairRes.RegimeResult}
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          {#if pairRes.FeatureImportances && pairRes.FeatureImportances.length > 0}
            <div
              class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-xs space-y-2 text-xs"
            >
              <div class="font-semibold text-slate-700 dark:text-slate-200">
                {$_("Correlation.mlpFeatureImportance")}
              </div>
              <div class="space-y-1.5">
                {#each pairRes.FeatureImportances.slice(0, 6) as fi}
                  <div class="flex items-center gap-2">
                    <span class="w-32 truncate text-[11px] text-slate-600 dark:text-slate-300 font-mono" title={fi.Feature}>
                      {fi.Feature}
                    </span>
                    <div class="flex-1 h-2 bg-slate-100 dark:bg-slate-800 rounded overflow-hidden">
                      <div
                        class="h-full bg-blue-500 rounded"
                        style="width: {Math.min(100, Math.max(2, fi.Importance * 100)).toFixed(1)}%"
                      ></div>
                    </div>
                    <span class="w-12 text-right text-[11px] font-mono text-slate-600 dark:text-slate-300">
                      {(fi.Importance * 100).toFixed(1)}%
                    </span>
                  </div>
                {/each}
              </div>
            </div>
          {/if}

          {#if pairRes.RegimeResult}
            <div
              class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-xs space-y-2 text-xs"
            >
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="font-semibold text-slate-700 dark:text-slate-200">
                  {$_("Correlation.knnRegimeMatrix")} ({pairRes.RegimeResult.TargetVar})
                </span>
                <span class="text-[11px] text-slate-500 dark:text-slate-400 font-mono">
                  Acc: <b>{(pairRes.RegimeResult.Accuracy * 100).toFixed(1)}%</b> |
                  BalAcc: <b>{(pairRes.RegimeResult.BalancedAccuracy * 100).toFixed(1)}%</b>
                </span>
              </div>
              <div class="overflow-x-auto">
                <table class="w-full text-[11px] border-collapse">
                  <thead>
                    <tr class="text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-700">
                      <th class="py-1 px-2 text-left">{$_("Correlation.actualPredicted")}</th>
                      {#each pairRes.RegimeResult.Labels || [] as lbl}
                        <th class="py-1 px-2 text-center">{lbl}</th>
                      {/each}
                    </tr>
                  </thead>
                  <tbody>
                    {#each pairRes.RegimeResult.Confusion || [] as row, rIdx}
                      <tr class="border-b border-slate-100 dark:border-slate-800">
                        <td class="py-1 px-2 font-medium text-slate-600 dark:text-slate-300">
                          {pairRes.RegimeResult.Labels?.[rIdx] || rIdx}
                        </td>
                        {#each row as cell, cIdx}
                          <td
                            class="py-1 px-2 text-center font-mono {rIdx === cIdx
                              ? 'font-bold text-blue-600 dark:text-blue-400 bg-blue-50/60 dark:bg-blue-900/20'
                              : 'text-slate-600 dark:text-slate-400'}"
                          >
                            {cell}
                          </td>
                        {/each}
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </div>
          {/if}
        </div>
      {/if}

      <!-- Pair Chart Panel -->
      <div
        class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-xs flex flex-col gap-3"
      >
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex flex-wrap items-center gap-1 bg-slate-100 dark:bg-slate-800 p-1 rounded-lg text-xs">
            <button
              type="button"
              onclick={() => (pairChartMode = "overlay")}
              class="flex items-center gap-1 px-2.5 py-1 rounded-md {pairChartMode === 'overlay'
                ? 'bg-white dark:bg-slate-700 text-blue-600 dark:text-blue-400 shadow-xs font-medium'
                : 'text-slate-600 dark:text-slate-400'}"
            >
              <TrendingUp class="w-3.5 h-3.5" />
              {$_("Correlation.chartOverlay")}
            </button>
            <button
              type="button"
              onclick={() => (pairChartMode = "scatter")}
              class="flex items-center gap-1 px-2.5 py-1 rounded-md {pairChartMode === 'scatter'
                ? 'bg-white dark:bg-slate-700 text-blue-600 dark:text-blue-400 shadow-xs font-medium'
                : 'text-slate-600 dark:text-slate-400'}"
            >
              <Activity class="w-3.5 h-3.5" />
              {$_("Correlation.chartScatter")}
            </button>
            <button
              type="button"
              onclick={() => (pairChartMode = "lag")}
              class="flex items-center gap-1 px-2.5 py-1 rounded-md {pairChartMode === 'lag'
                ? 'bg-white dark:bg-slate-700 text-blue-600 dark:text-blue-400 shadow-xs font-medium'
                : 'text-slate-600 dark:text-slate-400'}"
            >
              <BarChart3 class="w-3.5 h-3.5" />
              {$_("Correlation.chartLag")}
            </button>
            <button
              type="button"
              onclick={() => (pairChartMode = "matrix")}
              class="flex items-center gap-1 px-2.5 py-1 rounded-md {pairChartMode === 'matrix'
                ? 'bg-white dark:bg-slate-700 text-blue-600 dark:text-blue-400 shadow-xs font-medium'
                : 'text-slate-600 dark:text-slate-400'}"
            >
              <Grid3x3 class="w-3.5 h-3.5" />
              {$_("Correlation.chartMatrix")}
            </button>
          </div>

          {#if pairChartMode === "overlay"}
            <div class="flex items-center gap-1 bg-slate-100 dark:bg-slate-800 p-1 rounded-lg text-xs">
              <button
                type="button"
                onclick={() => (overlayMode = "dual")}
                class="px-2.5 py-1 rounded-md {overlayMode === 'dual'
                  ? 'bg-white dark:bg-slate-700 text-blue-600 dark:text-blue-400 shadow-xs font-medium'
                  : 'text-slate-600 dark:text-slate-400'}"
              >
                {$_("Correlation.overlayDual")}
              </button>
              <button
                type="button"
                onclick={() => (overlayMode = "norm")}
                class="px-2.5 py-1 rounded-md {overlayMode === 'norm'
                  ? 'bg-white dark:bg-slate-700 text-blue-600 dark:text-blue-400 shadow-xs font-medium'
                  : 'text-slate-600 dark:text-slate-400'}"
              >
                {$_("Correlation.overlayNorm")}
              </button>
            </div>
          {/if}
        </div>

        <div bind:this={pairChartDom} class="w-full h-96"></div>
      </div>
    {:else if !analyzing}
      <div
        class="p-10 text-center text-sm text-slate-500 dark:text-slate-400 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl"
      >
        {$_("Correlation.selectTwoPollings")}
      </div>
    {/if}
  {/if}
</div>

{#if showAiExplainModal}
  <ReportAIExplainModal
    bind:show={showAiExplainModal}
    reportType="correlation"
    title={$_("Correlation.title")}
    summary={aiSummary}
    items={aiItems}
  />
{/if}
