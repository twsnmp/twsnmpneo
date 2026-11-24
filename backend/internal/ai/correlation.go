package ai

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"gonum.org/v1/gonum/stat/distuv"
)

// CorrelationAlgorithm identifies one of the supported correlation algorithms.
type CorrelationAlgorithm string

const (
	CorrPearson   CorrelationAlgorithm = "pearson"
	CorrSpearman  CorrelationAlgorithm = "spearman"
	CorrKendall   CorrelationAlgorithm = "kendall"
	CorrCrossCorr CorrelationAlgorithm = "crosscorr"
	CorrDistance  CorrelationAlgorithm = "dcor"
)

// CorrelationPairReq requests correlation analysis between two pollings.
type CorrelationPairReq struct {
	PollingID1   string   `json:"PollingID1"`
	PollingID2   string   `json:"PollingID2"`
	Vars1        []string `json:"Vars1"`
	Vars2        []string `json:"Vars2"`
	StartTime    int64    `json:"StartTime"`    // Unix seconds (0 = default 24h ago)
	EndTime      int64    `json:"EndTime"`      // Unix seconds (0 = now)
	BucketSec    int      `json:"BucketSec"`    // 0 = auto
	Algorithm    string   `json:"Algorithm"`    // pearson|spearman|kendall|crosscorr|dcor
	MaxLag       int      `json:"MaxLag"`       // lag buckets (default 12)
	IncludeState bool     `json:"IncludeState"` // include polling state (normal=1, unknown=0.5, else=0)
}

// CorrelationVarPairScore represents correlation for a single (Var1, Var2) pair.
type CorrelationVarPairScore struct {
	Var1    string  `json:"Var1"`
	Var2    string  `json:"Var2"`
	Score   float64 `json:"Score"`
	PValue  float64 `json:"PValue"`
	Samples int     `json:"Samples"`
}

// CorrelationLagPoint represents cross-correlation at a given lag bucket.
type CorrelationLagPoint struct {
	LagBuckets int     `json:"LagBuckets"`
	LagSeconds int     `json:"LagSeconds"`
	Score      float64 `json:"Score"`
	Samples    int     `json:"Samples"`
}

// CorrelationSeriesPoint represents an aligned time-bucket data point for plotting.
type CorrelationSeriesPoint struct {
	Time   int64   `json:"Time"` // Unix seconds
	Value1 float64 `json:"Value1"`
	Value2 float64 `json:"Value2"`
}

// CorrelationPairRes holds the full comparison results between two pollings.
type CorrelationPairRes struct {
	PollingID1      string                    `json:"PollingID1"`
	PollingName1    string                    `json:"PollingName1"`
	NodeID1         string                    `json:"NodeID1"`
	NodeName1       string                    `json:"NodeName1"`
	PollingID2      string                    `json:"PollingID2"`
	PollingName2    string                    `json:"PollingName2"`
	NodeID2         string                    `json:"NodeID2"`
	NodeName2       string                    `json:"NodeName2"`
	StartTime       int64                     `json:"StartTime"`
	EndTime         int64                     `json:"EndTime"`
	BucketSec       int                       `json:"BucketSec"`
	Algorithm       string                    `json:"Algorithm"`
	AllVars1        []string                  `json:"AllVars1"`
	AllVars2        []string                  `json:"AllVars2"`
	UsedVars1       []string                  `json:"UsedVars1"`
	UsedVars2       []string                  `json:"UsedVars2"`
	BestVar1        string                    `json:"BestVar1"`
	BestVar2        string                    `json:"BestVar2"`
	SampleCount     int                       `json:"SampleCount"`
	Pearson         float64                   `json:"Pearson"`
	PearsonP        float64                   `json:"PearsonP"`
	Spearman        float64                   `json:"Spearman"`
	SpearmanP       float64                   `json:"SpearmanP"`
	Kendall         float64                   `json:"Kendall"`
	KendallP        float64                   `json:"KendallP"`
	CrossCorr       float64                   `json:"CrossCorr"`
	CrossCorrP      float64                   `json:"CrossCorrP"`
	BestLagBuckets  int                       `json:"BestLagBuckets"`
	BestLagSeconds  int                       `json:"BestLagSeconds"`
	DistanceCorr    float64                   `json:"DistanceCorr"`
	DistanceCorrP   float64                   `json:"DistanceCorrP"`
	Slope           float64                   `json:"Slope"`
	Intercept       float64                   `json:"Intercept"`
	Series          []CorrelationSeriesPoint  `json:"Series"`
	NormSeries      []CorrelationSeriesPoint  `json:"NormSeries"`
	LagProfile      []CorrelationLagPoint     `json:"LagProfile"`
	VarMatrix       []CorrelationVarPairScore `json:"VarMatrix"`
	Error           string                    `json:"Error,omitempty"`
}

// CorrelationDiscoverReq requests automatic discovery of correlated polling pairs.
type CorrelationDiscoverReq struct {
	BasePollingID string  `json:"BasePollingID"` // empty = all pairs
	StartTime     int64   `json:"StartTime"`
	EndTime       int64   `json:"EndTime"`
	BucketSec     int     `json:"BucketSec"`
	Algorithm     string  `json:"Algorithm"`
	MaxLag        int     `json:"MaxLag"`
	MinAbsScore   float64 `json:"MinAbsScore"`
	MinSamples    int     `json:"MinSamples"`
	CrossNodeOnly bool    `json:"CrossNodeOnly"`
	IncludeState  bool    `json:"IncludeState"`
	Limit         int     `json:"Limit"`
}

// CorrelationDiscoverItem represents a single discovered correlated polling pair.
type CorrelationDiscoverItem struct {
	PollingID1   string  `json:"PollingID1"`
	PollingName1 string  `json:"PollingName1"`
	NodeID1      string  `json:"NodeID1"`
	NodeName1    string  `json:"NodeName1"`
	PollingID2   string  `json:"PollingID2"`
	PollingName2 string  `json:"PollingName2"`
	NodeID2      string  `json:"NodeID2"`
	NodeName2    string  `json:"NodeName2"`
	SameNode     bool    `json:"SameNode"`
	BestVar1     string  `json:"BestVar1"`
	BestVar2     string  `json:"BestVar2"`
	Algorithm    string  `json:"Algorithm"`
	Score        float64 `json:"Score"`
	AbsScore     float64 `json:"AbsScore"`
	PValue       float64 `json:"PValue"`
	BestLagSec   int     `json:"BestLagSec"`
	Samples      int     `json:"Samples"`
}

// CorrelationDiscoverRes holds the discovery results and statistics.
type CorrelationDiscoverRes struct {
	StartTime      int64                     `json:"StartTime"`
	EndTime        int64                     `json:"EndTime"`
	BucketSec      int                       `json:"BucketSec"`
	Algorithm      string                    `json:"Algorithm"`
	TotalPollings  int                       `json:"TotalPollings"`
	PairsEvaluated int                       `json:"PairsEvaluated"`
	Canceled       bool                      `json:"Canceled"`
	Items          []CorrelationDiscoverItem `json:"Items"`
	Error          string                    `json:"Error,omitempty"`
}

// CorrelationCandidate represents a polling eligible for correlation analysis.
type CorrelationCandidate struct {
	PollingID   string   `json:"PollingID"`
	PollingName string   `json:"PollingName"`
	NodeID      string   `json:"NodeID"`
	NodeName    string   `json:"NodeName"`
	Type        string   `json:"Type"`
	Mode        string   `json:"Mode"`
	PollInt     int      `json:"PollInt"`
	Vars        []string `json:"Vars"`
}

// CorrelationProgress reports discovery progress to optional callbacks.
type CorrelationProgress struct {
	Phase   string `json:"Phase"` // loading | calculating | done | canceled
	Current int    `json:"Current"`
	Total   int    `json:"Total"`
	Found   int    `json:"Found"`
}

// GetCorrelationCandidates returns pollings that record logs along with their numeric variable keys.
func GetCorrelationCandidates(ctx context.Context, store datastore.DataStore, logStore *parquet.Store) []CorrelationCandidate {
	if store == nil {
		return nil
	}
	nodes, err := store.ListNodes(ctx)
	if err != nil {
		return nil
	}
	nodeMap := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n.Name
	}
	pollings, err := store.ListPollings(ctx)
	if err != nil {
		return nil
	}
	var res []CorrelationCandidate
	for _, p := range pollings {
		if p.LogMode == datastore.LogModeNone {
			continue
		}
		vars := getCorrelationKeys(ctx, logStore, p)
		res = append(res, CorrelationCandidate{
			PollingID:   p.ID,
			PollingName: p.Name,
			NodeID:      p.NodeID,
			NodeName:    nodeMap[p.NodeID],
			Type:        p.Type,
			Mode:        p.Mode,
			PollInt:     p.PollInt,
			Vars:        vars,
		})
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].NodeName != res[j].NodeName {
			return res[i].NodeName < res[j].NodeName
		}
		return res[i].PollingName < res[j].PollingName
	})
	return res
}

func resolveCorrelationTimeRange(st, et int64) (int64, int64) {
	now := time.Now().Unix()
	if et <= 0 {
		et = now
	}
	if st <= 0 {
		st = et - 24*3600
	}
	if st >= et {
		st = et - 3600
	}
	return st, et
}

func resolveAutoBucketSec(st, et int64, pollInt1, pollInt2 int) int {
	dur := et - st
	var bucket int
	switch {
	case dur <= 24*3600:
		bucket = 60
	case dur <= 7*24*3600:
		bucket = 300
	case dur <= 30*24*3600:
		bucket = 900
	default:
		bucket = 3600
	}
	maxPoll := pollInt1
	if pollInt2 > maxPoll {
		maxPoll = pollInt2
	}
	if maxPoll > bucket && maxPoll <= 3600 {
		bucket = maxPoll
	}
	if bucket < 10 {
		bucket = 60
	}
	return bucket
}

func normalizeAlgorithm(algo string) CorrelationAlgorithm {
	switch strings.ToLower(strings.TrimSpace(algo)) {
	case string(CorrSpearman):
		return CorrSpearman
	case string(CorrKendall):
		return CorrKendall
	case string(CorrCrossCorr):
		return CorrCrossCorr
	case string(CorrDistance):
		return CorrDistance
	default:
		return CorrPearson
	}
}

func stateToNumeric(state string) float64 {
	switch strings.ToLower(state) {
	case "normal", "repair":
		return 1.0
	case "unknown":
		return 0.5
	default:
		return 0.0
	}
}

func corrToFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return 0, false
		}
		return val, true
	case float32:
		f := float64(val)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint64:
		return float64(val), true
	case uint:
		return float64(val), true
	case bool:
		if val {
			return 1.0, true
		}
		return 0.0, true
	}
	return 0, false
}

func getCorrelationKeys(ctx context.Context, logStore *parquet.Store, p *datastore.PollingEnt) []string {
	if p == nil {
		return nil
	}
	if p.Type == "syslog" && p.Mode == "pri" {
		keys := make([]string, 256)
		for i := 0; i < 256; i++ {
			keys[i] = fmt.Sprintf("pri_%d", i)
		}
		return keys
	}
	var vecKeys []string
	if p.VectorCols != "" {
		for _, k := range strings.Split(p.VectorCols, ",") {
			k = strings.TrimSpace(k)
			if k != "" && k != "hour" && k != "weekday" {
				vecKeys = append(vecKeys, k)
			}
		}
	}
	keySet := make(map[string]bool)
	for k, v := range p.Result {
		if _, ok := corrToFloat64(v); ok {
			keySet[k] = true
		}
	}
	if len(keySet) == 0 && logStore != nil {
		if logs, err := logStore.GetRecentPollingLog(ctx, p.ID, 20); err == nil {
			for _, l := range logs {
				for k, v := range l.Result {
					if _, ok := corrToFloat64(v); ok {
						keySet[k] = true
					}
				}
			}
		}
	}
	if len(vecKeys) > 0 && len(keySet) > 0 {
		var filtered []string
		for _, k := range vecKeys {
			if keySet[k] {
				filtered = append(filtered, k)
			}
		}
		if len(filtered) > 0 {
			return filtered
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// bucketSeries maps bucketSec-aligned Unix second timestamps to variable values.
type bucketSeries struct {
	Vars       []string
	Timestamps []int64              // sorted ascending
	Data       map[int64][]float64  // ts -> values matching Vars order
	HasVal     map[int64][]bool     // ts -> validity per Var
}

func buildBucketSeriesFromLogs(logs []*datastore.PollingLogEnt, vars []string, bucketSec int, includeState bool) *bucketSeries {
	varList := make([]string, 0, len(vars)+1)
	seen := make(map[string]bool, len(vars)+1)
	for _, v := range vars {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		varList = append(varList, v)
	}
	if includeState && !seen["state"] {
		varList = append(varList, "state")
	}

	bs := &bucketSeries{
		Vars:   varList,
		Data:   make(map[int64][]float64),
		HasVal: make(map[int64][]bool),
	}
	if len(varList) == 0 || len(logs) == 0 || bucketSec <= 0 {
		return bs
	}

	type acc struct {
		sums   []float64
		counts []int
	}
	buckets := make(map[int64]*acc)
	bSec := int64(bucketSec)

	for _, l := range logs {
		sec := l.Time / 1_000_000_000
		b := (sec / bSec) * bSec
		a, ok := buckets[b]
		if !ok {
			a = &acc{
				sums:   make([]float64, len(varList)),
				counts: make([]int, len(varList)),
			}
			buckets[b] = a
		}
		for idx, vk := range varList {
			if vk == "state" {
				a.sums[idx] += stateToNumeric(l.State)
				a.counts[idx]++
				continue
			}
			if l.Result != nil {
				if raw, exists := l.Result[vk]; exists {
					if f, ok := corrToFloat64(raw); ok {
						a.sums[idx] += f
						a.counts[idx]++
					}
				}
			}
		}
	}

	tsList := make([]int64, 0, len(buckets))
	for b, a := range buckets {
		vals := make([]float64, len(varList))
		has := make([]bool, len(varList))
		anyValid := false
		for i := range varList {
			if a.counts[i] > 0 {
				vals[i] = a.sums[i] / float64(a.counts[i])
				has[i] = true
				anyValid = true
			}
		}
		if anyValid {
			tsList = append(tsList, b)
			bs.Data[b] = vals
			bs.HasVal[b] = has
		}
	}
	sort.Slice(tsList, func(i, j int) bool { return tsList[i] < tsList[j] })
	bs.Timestamps = tsList
	return bs
}

// alignTwoBucketSeries intersects timestamps from bs1 and bs2 and returns aligned column slices.
func alignTwoBucketSeries(bs1, bs2 *bucketSeries) (timestamps []int64, mat1 [][]float64, mat2 [][]float64) {
	if bs1 == nil || bs2 == nil || len(bs1.Timestamps) == 0 || len(bs2.Timestamps) == 0 {
		return nil, nil, nil
	}
	nVars1 := len(bs1.Vars)
	nVars2 := len(bs2.Vars)
	if nVars1 == 0 || nVars2 == 0 {
		return nil, nil, nil
	}

	mat1 = make([][]float64, nVars1)
	mat2 = make([][]float64, nVars2)

	i, j := 0, 0
	for i < len(bs1.Timestamps) && j < len(bs2.Timestamps) {
		t1 := bs1.Timestamps[i]
		t2 := bs2.Timestamps[j]
		if t1 == t2 {
			v1 := bs1.Data[t1]
			h1 := bs1.HasVal[t1]
			v2 := bs2.Data[t2]
			h2 := bs2.HasVal[t2]
			ok := true
			for k := 0; k < nVars1; k++ {
				if !h1[k] {
					ok = false
					break
				}
			}
			if ok {
				for k := 0; k < nVars2; k++ {
					if !h2[k] {
						ok = false
						break
					}
				}
			}
			if ok {
				timestamps = append(timestamps, t1)
				for k := 0; k < nVars1; k++ {
					mat1[k] = append(mat1[k], v1[k])
				}
				for k := 0; k < nVars2; k++ {
					mat2[k] = append(mat2[k], v2[k])
				}
			}
			i++
			j++
		} else if t1 < t2 {
			i++
		} else {
			j++
		}
	}
	return timestamps, mat1, mat2
}

// pearsonCorr computes Pearson correlation r and two-tailed p-value using Student's t-distribution.
func pearsonCorr(x, y []float64) (float64, float64) {
	n := len(x)
	if n < 3 || len(y) != n {
		return 0, 1
	}
	var sumX, sumY float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / float64(n)
	meanY := sumY / float64(n)
	var num, dx2, dy2 float64
	for i := 0; i < n; i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		num += dx * dy
		dx2 += dx * dx
		dy2 += dy * dy
	}
	den := math.Sqrt(dx2 * dy2)
	if den <= 1e-15 {
		return 0, 1
	}
	r := num / den
	if r > 1 {
		r = 1
	} else if r < -1 {
		r = -1
	}
	p := correlationPValue(r, n)
	return r, p
}

func correlationPValue(r float64, n int) float64 {
	if n <= 2 {
		return 1
	}
	absR := math.Abs(r)
	if absR >= 1.0-1e-12 {
		return 0
	}
	df := float64(n - 2)
	tVal := absR * math.Sqrt(df/(1.0-r*r))
	tDist := distuv.StudentsT{Mu: 0, Sigma: 1, Nu: df}
	p := 2.0 * (1.0 - tDist.CDF(tVal))
	if p < 0 {
		p = 0
	} else if p > 1 {
		p = 1
	}
	return p
}

func averageRanks(a []float64) []float64 {
	n := len(a)
	idx := make([]int, n)
	for i := 0; i < n; i++ {
		idx[i] = i
	}
	sort.Slice(idx, func(i, j int) bool {
		return a[idx[i]] < a[idx[j]]
	})
	ranks := make([]float64, n)
	i := 0
	for i < n {
		j := i + 1
		for j < n && a[idx[j]] == a[idx[i]] {
			j++
		}
		avgRank := float64(i+j-1)/2.0 + 1.0
		for k := i; k < j; k++ {
			ranks[idx[k]] = avgRank
		}
		i = j
	}
	return ranks
}

func spearmanCorr(x, y []float64) (float64, float64) {
	n := len(x)
	if n < 3 || len(y) != n {
		return 0, 1
	}
	rx := averageRanks(x)
	ry := averageRanks(y)
	return pearsonCorr(rx, ry)
}

func subsamplePair(x, y []float64, maxN int) ([]float64, []float64) {
	n := len(x)
	if n <= maxN || maxN <= 0 {
		return x, y
	}
	sx := make([]float64, maxN)
	sy := make([]float64, maxN)
	for i := 0; i < maxN; i++ {
		idx := int(float64(i) * float64(n-1) / float64(maxN-1))
		sx[i] = x[idx]
		sy[i] = y[idx]
	}
	return sx, sy
}

func kendallTauB(x, y []float64) (float64, float64) {
	n := len(x)
	if n < 3 || len(y) != n {
		return 0, 1
	}
	xs, ys := subsamplePair(x, y, 1000)
	m := len(xs)
	var concordant, discordant, tiesX, tiesY int64
	for i := 0; i < m-1; i++ {
		for j := i + 1; j < m; j++ {
			dx := xs[i] - xs[j]
			dy := ys[i] - ys[j]
			if dx == 0 && dy == 0 {
				continue
			} else if dx == 0 {
				tiesX++
			} else if dy == 0 {
				tiesY++
			} else if (dx > 0 && dy > 0) || (dx < 0 && dy < 0) {
				concordant++
			} else {
				discordant++
			}
		}
	}
	n0 := float64(int64(m) * int64(m-1) / 2)
	den := math.Sqrt((n0 - float64(tiesX)) * (n0 - float64(tiesY)))
	if den <= 1e-15 {
		return 0, 1
	}
	tau := float64(concordant-discordant) / den
	if tau > 1 {
		tau = 1
	} else if tau < -1 {
		tau = -1
	}
	mf := float64(m)
	variance := (2.0 * (2.0*mf + 5.0)) / (9.0 * mf * (mf - 1.0))
	if variance <= 0 {
		return tau, 1
	}
	z := math.Abs(tau) / math.Sqrt(variance)
	norm := distuv.Normal{Mu: 0, Sigma: 1}
	p := 2.0 * (1.0 - norm.CDF(z))
	if p < 0 {
		p = 0
	} else if p > 1 {
		p = 1
	}
	return tau, p
}

// crossCorrLag computes lagged Pearson correlation across [-maxLag, +maxLag].
// Positive lag k means y[t+k] correlates with x[t] (x leads y by k buckets).
func crossCorrLag(x, y []float64, maxLag, bucketSec int) (bestScore, bestP float64, bestLag int, profile []CorrelationLagPoint) {
	n := len(x)
	if n < 5 || len(y) != n {
		return 0, 1, 0, nil
	}
	if maxLag <= 0 {
		maxLag = 12
	}
	if maxLag >= n-3 {
		maxLag = n - 4
	}
	if maxLag < 0 {
		maxLag = 0
	}
	profile = make([]CorrelationLagPoint, 0, 2*maxLag+1)
	bestAbs := -1.0
	bestP = 1.0
	for lag := -maxLag; lag <= maxLag; lag++ {
		var sx, sy []float64
		if lag >= 0 {
			sx = x[:n-lag]
			sy = y[lag:]
		} else {
			k := -lag
			sx = x[k:]
			sy = y[:n-k]
		}
		if len(sx) < 4 {
			continue
		}
		r, p := pearsonCorr(sx, sy)
		profile = append(profile, CorrelationLagPoint{
			LagBuckets: lag,
			LagSeconds: lag * bucketSec,
			Score:      r,
			Samples:    len(sx),
		})
		if math.Abs(r) > bestAbs {
			bestAbs = math.Abs(r)
			bestScore = r
			bestP = p
			bestLag = lag
		}
	}
	return bestScore, bestP, bestLag, profile
}

func subsampleColumns(cols [][]float64, maxN int) [][]float64 {
	if len(cols) == 0 {
		return cols
	}
	n := len(cols[0])
	if n <= maxN || maxN <= 0 {
		return cols
	}
	res := make([][]float64, len(cols))
	for c := range cols {
		res[c] = make([]float64, maxN)
		for i := 0; i < maxN; i++ {
			idx := int(float64(i) * float64(n-1) / float64(maxN-1))
			res[c][i] = cols[c][idx]
		}
	}
	return res
}

func standardizeColumns(cols [][]float64) [][]float64 {
	res := make([][]float64, len(cols))
	for c, col := range cols {
		n := len(col)
		out := make([]float64, n)
		if n == 0 {
			res[c] = out
			continue
		}
		var sum float64
		for _, v := range col {
			sum += v
		}
		mean := sum / float64(n)
		var sq float64
		for _, v := range col {
			d := v - mean
			sq += d * d
		}
		std := math.Sqrt(sq / float64(n))
		if std <= 1e-12 {
			res[c] = out
			continue
		}
		for i, v := range col {
			out[i] = (v - mean) / std
		}
		res[c] = out
	}
	return res
}

func doubleCenteredDistMatrix(cols [][]float64) [][]float64 {
	p := len(cols)
	if p == 0 {
		return nil
	}
	n := len(cols[0])
	if n == 0 {
		return nil
	}
	a := make([][]float64, n)
	rowMean := make([]float64, n)
	var grandSum float64
	for i := 0; i < n; i++ {
		a[i] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			var sq float64
			for c := 0; c < p; c++ {
				d := cols[c][i] - cols[c][j]
				sq += d * d
			}
			dist := math.Sqrt(sq)
			a[i][j] = dist
			a[j][i] = dist
		}
	}
	for i := 0; i < n; i++ {
		var rSum float64
		for j := 0; j < n; j++ {
			rSum += a[i][j]
		}
		rowMean[i] = rSum / float64(n)
		grandSum += rSum
	}
	grandMean := grandSum / float64(n*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			a[i][j] = a[i][j] - rowMean[i] - rowMean[j] + grandMean
		}
	}
	return a
}

// distanceCorrMatrix computes multivariate Distance Correlation in [0, 1] and optional permutation p-value.
func distanceCorrMatrix(cols1, cols2 [][]float64, withPValue bool) (float64, float64) {
	if len(cols1) == 0 || len(cols2) == 0 {
		return 0, 1
	}
	n := len(cols1[0])
	if n < 5 || len(cols2[0]) != n {
		return 0, 1
	}
	const maxDCorN = 200
	if n > maxDCorN {
		cols1 = subsampleColumns(cols1, maxDCorN)
		cols2 = subsampleColumns(cols2, maxDCorN)
		n = maxDCorN
	}
	z1 := standardizeColumns(cols1)
	z2 := standardizeColumns(cols2)

	A := doubleCenteredDistMatrix(z1)
	B := doubleCenteredDistMatrix(z2)
	if A == nil || B == nil {
		return 0, 1
	}

	var dcov2, dvar1, dvar2 float64
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			dcov2 += A[i][j] * B[i][j]
			dvar1 += A[i][j] * A[i][j]
			dvar2 += B[i][j] * B[i][j]
		}
	}
	nf := float64(n * n)
	dcov2 /= nf
	dvar1 /= nf
	dvar2 /= nf
	if dvar1 <= 1e-18 || dvar2 <= 1e-18 || dcov2 <= 0 {
		return 0, 1
	}
	dcor := math.Sqrt(dcov2) / math.Sqrt(math.Sqrt(dvar1*dvar2))
	if dcor > 1 {
		dcor = 1
	} else if dcor < 0 {
		dcor = 0
	}
	if !withPValue {
		return dcor, 1
	}

	const perms = 99
	rng := rand.New(rand.NewSource(42))
	idx := make([]int, n)
	for i := 0; i < n; i++ {
		idx[i] = i
	}
	geCount := 0
	for p := 0; p < perms; p++ {
		rng.Shuffle(n, func(i, j int) {
			idx[i], idx[j] = idx[j], idx[i]
		})
		var pdcov2 float64
		for i := 0; i < n; i++ {
			pi := idx[i]
			for j := 0; j < n; j++ {
				pdcov2 += A[i][j] * B[pi][idx[j]]
			}
		}
		pdcov2 /= nf
		if pdcov2 > 0 {
			pdcor := math.Sqrt(pdcov2) / math.Sqrt(math.Sqrt(dvar1*dvar2))
			if pdcor >= dcor {
				geCount++
			}
		}
	}
	pval := float64(geCount+1) / float64(perms+1)
	return dcor, pval
}

func linearRegression(x, y []float64) (slope, intercept float64) {
	n := len(x)
	if n < 2 || len(y) != n {
		return 0, 0
	}
	var sumX, sumY float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
	}
	mx := sumX / float64(n)
	my := sumY / float64(n)
	var num, den float64
	for i := 0; i < n; i++ {
		dx := x[i] - mx
		num += dx * (y[i] - my)
		den += dx * dx
	}
	if den <= 1e-15 {
		return 0, my
	}
	slope = num / den
	intercept = my - slope*mx
	return slope, intercept
}

func normalizeMinMax(a []float64) []float64 {
	n := len(a)
	out := make([]float64, n)
	if n == 0 {
		return out
	}
	minV, maxV := a[0], a[0]
	for _, v := range a[1:] {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	span := maxV - minV
	if span <= 1e-15 {
		for i := range out {
			out[i] = 0.5
		}
		return out
	}
	for i, v := range a {
		out[i] = (v - minV) / span
	}
	return out
}

func resolveSelectedVars(allVars, requested []string, includeState bool) []string {
	var out []string
	seen := make(map[string]bool)
	if len(requested) > 0 {
		avail := make(map[string]bool, len(allVars)+1)
		for _, v := range allVars {
			avail[v] = true
		}
		if includeState {
			avail["state"] = true
		}
		for _, r := range requested {
			if avail[r] && !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	if len(out) == 0 {
		for _, v := range allVars {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
		if includeState && !seen["state"] {
			out = append(out, "state")
		}
	}
	return out
}

// AnalyzePollingCorrelation computes detailed correlation metrics and aligned series between two pollings.
func AnalyzePollingCorrelation(ctx context.Context, store datastore.DataStore, logStore *parquet.Store, req CorrelationPairReq) CorrelationPairRes {
	res := CorrelationPairRes{
		PollingID1: req.PollingID1,
		PollingID2: req.PollingID2,
		Algorithm:  string(normalizeAlgorithm(req.Algorithm)),
	}
	if store == nil || logStore == nil {
		res.Error = "datastore or log store is not initialized"
		return res
	}
	if req.PollingID1 == "" || req.PollingID2 == "" {
		res.Error = "two polling IDs are required"
		return res
	}
	p1, err1 := store.GetPolling(ctx, req.PollingID1)
	p2, err2 := store.GetPolling(ctx, req.PollingID2)
	if err1 != nil || p1 == nil || err2 != nil || p2 == nil {
		res.Error = "polling not found"
		return res
	}
	n1, _ := store.GetNode(ctx, p1.NodeID)
	n2, _ := store.GetNode(ctx, p2.NodeID)

	res.PollingName1 = p1.Name
	res.NodeID1 = p1.NodeID
	if n1 != nil {
		res.NodeName1 = n1.Name
	}
	res.PollingName2 = p2.Name
	res.NodeID2 = p2.NodeID
	if n2 != nil {
		res.NodeName2 = n2.Name
	}

	st, et := resolveCorrelationTimeRange(req.StartTime, req.EndTime)
	res.StartTime = st
	res.EndTime = et

	bucketSec := req.BucketSec
	if bucketSec <= 0 {
		bucketSec = resolveAutoBucketSec(st, et, p1.PollInt, p2.PollInt)
	}
	res.BucketSec = bucketSec

	maxLag := req.MaxLag
	if maxLag <= 0 {
		maxLag = 12
	}
	if maxLag > 60 {
		maxLag = 60
	}

	allVars1 := getCorrelationKeys(ctx, logStore, p1)
	allVars2 := getCorrelationKeys(ctx, logStore, p2)
	if req.IncludeState {
		hasS1 := false
		for _, v := range allVars1 {
			if v == "state" {
				hasS1 = true
				break
			}
		}
		if !hasS1 {
			allVars1 = append(allVars1, "state")
		}
		hasS2 := false
		for _, v := range allVars2 {
			if v == "state" {
				hasS2 = true
				break
			}
		}
		if !hasS2 {
			allVars2 = append(allVars2, "state")
		}
	}
	res.AllVars1 = allVars1
	res.AllVars2 = allVars2

	used1 := resolveSelectedVars(allVars1, req.Vars1, req.IncludeState)
	used2 := resolveSelectedVars(allVars2, req.Vars2, req.IncludeState)
	res.UsedVars1 = used1
	res.UsedVars2 = used2

	if len(used1) == 0 || len(used2) == 0 {
		res.Error = "no numeric variables available for one or both pollings"
		return res
	}

	logsMap := logStore.GetPollingLogsBatch(ctx, []string{p1.ID, p2.ID}, st*1_000_000_000, et*1_000_000_000, nil)
	bs1 := buildBucketSeriesFromLogs(logsMap[p1.ID], used1, bucketSec, req.IncludeState)
	bs2 := buildBucketSeriesFromLogs(logsMap[p2.ID], used2, bucketSec, req.IncludeState)

	ts, mat1, mat2 := alignTwoBucketSeries(bs1, bs2)
	res.SampleCount = len(ts)
	if len(ts) < 3 {
		res.Error = fmt.Sprintf("insufficient aligned samples (%d < 3)", len(ts))
		return res
	}

	algo := normalizeAlgorithm(req.Algorithm)
	bestIdx1, bestIdx2 := 0, 0
	bestAbs := -1.0

	varMatrix := make([]CorrelationVarPairScore, 0, len(used1)*len(used2))
	for i, v1 := range used1 {
		for j, v2 := range used2 {
			var sc, pv float64
			switch algo {
			case CorrSpearman:
				sc, pv = spearmanCorr(mat1[i], mat2[j])
			case CorrKendall:
				sc, pv = kendallTauB(mat1[i], mat2[j])
			case CorrCrossCorr:
				sc, pv, _, _ = crossCorrLag(mat1[i], mat2[j], maxLag, bucketSec)
			case CorrDistance:
				sc, pv = distanceCorrMatrix([][]float64{mat1[i]}, [][]float64{mat2[j]}, false)
			default:
				sc, pv = pearsonCorr(mat1[i], mat2[j])
			}
			varMatrix = append(varMatrix, CorrelationVarPairScore{
				Var1:    v1,
				Var2:    v2,
				Score:   sc,
				PValue:  pv,
				Samples: len(ts),
			})
			if math.Abs(sc) > bestAbs {
				bestAbs = math.Abs(sc)
				bestIdx1 = i
				bestIdx2 = j
			}
		}
	}
	res.VarMatrix = varMatrix
	res.BestVar1 = used1[bestIdx1]
	res.BestVar2 = used2[bestIdx2]

	bx := mat1[bestIdx1]
	by := mat2[bestIdx2]

	res.Pearson, res.PearsonP = pearsonCorr(bx, by)
	res.Spearman, res.SpearmanP = spearmanCorr(bx, by)
	res.Kendall, res.KendallP = kendallTauB(bx, by)
	res.CrossCorr, res.CrossCorrP, res.BestLagBuckets, res.LagProfile = crossCorrLag(bx, by, maxLag, bucketSec)
	res.BestLagSeconds = res.BestLagBuckets * bucketSec
	res.DistanceCorr, res.DistanceCorrP = distanceCorrMatrix(mat1, mat2, true)
	res.Slope, res.Intercept = linearRegression(bx, by)

	nx := normalizeMinMax(bx)
	ny := normalizeMinMax(by)
	res.Series = make([]CorrelationSeriesPoint, len(ts))
	res.NormSeries = make([]CorrelationSeriesPoint, len(ts))
	for k := range ts {
		res.Series[k] = CorrelationSeriesPoint{
			Time:   ts[k],
			Value1: bx[k],
			Value2: by[k],
		}
		res.NormSeries[k] = CorrelationSeriesPoint{
			Time:   ts[k],
			Value1: nx[k],
			Value2: ny[k],
		}
	}

	return res
}

// DiscoverPollingCorrelationsWithProgress scans all or base-relative polling pairs and returns sorted correlated pairs.
func DiscoverPollingCorrelationsWithProgress(ctx context.Context, store datastore.DataStore, logStore *parquet.Store, req CorrelationDiscoverReq, progressCb func(CorrelationProgress)) CorrelationDiscoverRes {
	emit := func(p CorrelationProgress) {
		if progressCb != nil {
			progressCb(p)
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	algo := normalizeAlgorithm(req.Algorithm)
	st, et := resolveCorrelationTimeRange(req.StartTime, req.EndTime)
	bucketSec := req.BucketSec
	if bucketSec <= 0 {
		bucketSec = resolveAutoBucketSec(st, et, 60, 60)
	}
	minSamples := req.MinSamples
	if minSamples < 3 {
		minSamples = 10
	}
	minAbs := req.MinAbsScore
	if minAbs <= 0 {
		minAbs = 0.5
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	maxLag := req.MaxLag
	if maxLag <= 0 {
		maxLag = 6
	}
	if maxLag > 30 {
		maxLag = 30
	}

	res := CorrelationDiscoverRes{
		StartTime: st,
		EndTime:   et,
		BucketSec: bucketSec,
		Algorithm: string(algo),
	}

	if store == nil || logStore == nil {
		res.Error = "datastore or log store is not initialized"
		return res
	}

	nodes, err := store.ListNodes(ctx)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	nodeMap := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n.Name
	}

	allPollings, err := store.ListPollings(ctx)
	if err != nil {
		res.Error = err.Error()
		return res
	}

	type pollingPrep struct {
		p    *datastore.PollingEnt
		vars []string
		bs   *bucketSeries
	}
	var preps []*pollingPrep
	var pollingIDs []string
	for _, p := range allPollings {
		if p.LogMode == datastore.LogModeNone {
			continue
		}
		vars := getCorrelationKeys(ctx, logStore, p)
		if len(vars) == 0 && !req.IncludeState {
			continue
		}
		if len(vars) > 8 {
			vars = vars[:8]
		}
		preps = append(preps, &pollingPrep{p: p, vars: vars})
		pollingIDs = append(pollingIDs, p.ID)
	}
	res.TotalPollings = len(preps)
	if len(preps) < 2 {
		return res
	}

	emit(CorrelationProgress{
		Phase:   "loading",
		Current: 0,
		Total:   1,
	})

	logsMap := logStore.GetPollingLogsBatch(ctx, pollingIDs, st*1_000_000_000, et*1_000_000_000, func(done, total int) {
		emit(CorrelationProgress{
			Phase:   "loading",
			Current: done,
			Total:   total,
		})
	})

	if ctx.Err() != nil {
		res.Canceled = true
		emit(CorrelationProgress{Phase: "canceled"})
		return res
	}

	var activePreps []*pollingPrep
	for _, pr := range preps {
		logs := logsMap[pr.p.ID]
		if len(logs) < minSamples {
			continue
		}
		pr.bs = buildBucketSeriesFromLogs(logs, pr.vars, bucketSec, req.IncludeState)
		if len(pr.bs.Timestamps) >= minSamples {
			activePreps = append(activePreps, pr)
		}
	}

	if len(activePreps) < 2 {
		emit(CorrelationProgress{Phase: "done"})
		return res
	}

	type pairIdx struct{ i, j int }
	var pairs []pairIdx
	if req.BasePollingID != "" {
		baseIdx := -1
		for idx, pr := range activePreps {
			if pr.p.ID == req.BasePollingID {
				baseIdx = idx
				break
			}
		}
		if baseIdx < 0 {
			res.Error = "base polling has insufficient log data in the selected range"
			return res
		}
		for j := range activePreps {
			if j == baseIdx {
				continue
			}
			if req.CrossNodeOnly && activePreps[baseIdx].p.NodeID == activePreps[j].p.NodeID {
				continue
			}
			pairs = append(pairs, pairIdx{baseIdx, j})
		}
	} else {
		for i := 0; i < len(activePreps)-1; i++ {
			for j := i + 1; j < len(activePreps); j++ {
				if req.CrossNodeOnly && activePreps[i].p.NodeID == activePreps[j].p.NodeID {
					continue
				}
				pairs = append(pairs, pairIdx{i, j})
			}
		}
	}

	totalPairs := len(pairs)
	if totalPairs == 0 {
		emit(CorrelationProgress{Phase: "done"})
		return res
	}

	emit(CorrelationProgress{
		Phase:   "calculating",
		Current: 0,
		Total:   totalPairs,
	})

	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if totalPairs < workers {
		workers = totalPairs
	}

	pairCh := make(chan pairIdx, totalPairs)
	for _, p := range pairs {
		pairCh <- p
	}
	close(pairCh)

	var mu sync.Mutex
	var items []CorrelationDiscoverItem
	var donePairs atomic.Int64
	var foundPairs atomic.Int64
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pr := range pairCh {
				if ctx.Err() != nil {
					return
				}
				p1 := activePreps[pr.i]
				p2 := activePreps[pr.j]
				ts, mat1, mat2 := alignTwoBucketSeries(p1.bs, p2.bs)
				if len(ts) >= minSamples {
					bestScore := 0.0
					bestAbs := -1.0
					bestP := 1.0
					bestLagSec := 0
					bestV1 := p1.bs.Vars[0]
					bestV2 := p2.bs.Vars[0]

					if algo == CorrDistance {
						sc, _ := distanceCorrMatrix(mat1, mat2, false)
						bestScore = sc
						bestAbs = math.Abs(sc)
						if len(p1.bs.Vars) > 1 || len(p2.bs.Vars) > 1 {
							bestV1 = strings.Join(p1.bs.Vars, ",")
							bestV2 = strings.Join(p2.bs.Vars, ",")
						}
					} else {
						for vi, v1 := range p1.bs.Vars {
							for vj, v2 := range p2.bs.Vars {
								var sc, pv float64
								var lagB int
								switch algo {
								case CorrSpearman:
									sc, pv = spearmanCorr(mat1[vi], mat2[vj])
								case CorrKendall:
									sc, pv = kendallTauB(mat1[vi], mat2[vj])
								case CorrCrossCorr:
									sc, pv, lagB, _ = crossCorrLag(mat1[vi], mat2[vj], maxLag, bucketSec)
								default:
									sc, pv = pearsonCorr(mat1[vi], mat2[vj])
								}
								if math.Abs(sc) > bestAbs {
									bestAbs = math.Abs(sc)
									bestScore = sc
									bestP = pv
									bestLagSec = lagB * bucketSec
									bestV1 = v1
									bestV2 = v2
								}
							}
						}
					}

					if bestAbs >= minAbs {
						item := CorrelationDiscoverItem{
							PollingID1:   p1.p.ID,
							PollingName1: p1.p.Name,
							NodeID1:      p1.p.NodeID,
							NodeName1:    nodeMap[p1.p.NodeID],
							PollingID2:   p2.p.ID,
							PollingName2: p2.p.Name,
							NodeID2:      p2.p.NodeID,
							NodeName2:    nodeMap[p2.p.NodeID],
							SameNode:     p1.p.NodeID == p2.p.NodeID,
							BestVar1:     bestV1,
							BestVar2:     bestV2,
							Algorithm:    string(algo),
							Score:        bestScore,
							AbsScore:     bestAbs,
							PValue:       bestP,
							BestLagSec:   bestLagSec,
							Samples:      len(ts),
						}
						mu.Lock()
						items = append(items, item)
						mu.Unlock()
						foundPairs.Add(1)
					}
				}
				cur := int(donePairs.Add(1))
				if cur%25 == 0 || cur == totalPairs {
					emit(CorrelationProgress{
						Phase:   "calculating",
						Current: cur,
						Total:   totalPairs,
						Found:   int(foundPairs.Load()),
					})
				}
			}
		}()
	}
	wg.Wait()

	if ctx.Err() != nil {
		res.Canceled = true
	}
	res.PairsEvaluated = int(donePairs.Load())

	sort.Slice(items, func(i, j int) bool {
		if items[i].AbsScore != items[j].AbsScore {
			return items[i].AbsScore > items[j].AbsScore
		}
		return items[i].Samples > items[j].Samples
	})
	if len(items) > limit {
		items = items[:limit]
	}
	res.Items = items

	if res.Canceled {
		emit(CorrelationProgress{Phase: "canceled", Current: res.PairsEvaluated, Total: totalPairs, Found: len(items)})
	} else {
		emit(CorrelationProgress{Phase: "done", Current: totalPairs, Total: totalPairs, Found: len(items)})
	}
	return res
}

// DiscoverPollingCorrelations runs correlation discovery without progress callbacks.
func DiscoverPollingCorrelations(ctx context.Context, store datastore.DataStore, logStore *parquet.Store, req CorrelationDiscoverReq) CorrelationDiscoverRes {
	return DiscoverPollingCorrelationsWithProgress(ctx, store, logStore, req, nil)
}
