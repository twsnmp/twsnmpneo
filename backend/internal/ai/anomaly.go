package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	go_iforest "github.com/codegaudi/go-iforest"
	tensai "github.com/mattn/tensai"
	"github.com/mattn/tensai/layer"
	"github.com/mattn/tensai/loss"
	"github.com/mattn/tensai/model"
	"github.com/mattn/tensai/optim"
	"github.com/montanaflynn/stats"
	"github.com/twsnmp/golof/lof"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
	"gonum.org/v1/gonum/mat"
)

// AIDataFrame represents the time series matrix of extracted features.
type AIDataFrame struct {
	Time []int64
	Data map[string][]float64
}

func (df AIDataFrame) Len() int {
	return len(df.Time)
}

func (df AIDataFrame) ColumnNames() []string {
	cols := make([]string, 0, len(df.Data))
	for k := range df.Data {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	return cols
}

func (df AIDataFrame) ToCSV(w io.Writer) error {
	cols := df.ColumnNames()
	row := "time"
	for _, k := range cols {
		row += "," + k
	}
	if _, err := w.Write([]byte(row + "\n")); err != nil {
		return err
	}
	for i, t := range df.Time {
		row = time.Unix(t, 0).Format(time.RFC3339)
		for _, col := range cols {
			row += ","
			if v, ok := df.Data[col]; ok && len(v) > i {
				row += fmt.Sprintf("%f", v[i])
			}
		}
		if _, err := w.Write([]byte(row + "\n")); err != nil {
			return err
		}
	}
	return nil
}

// AIReq wraps a request to calculate anomaly score for a polling.
type AIReq struct {
	PollingID string
	Df        AIDataFrame
}

func getStateNum(s string) float64 {
	switch strings.ToLower(s) {
	case "repair", "normal":
		return 1.0
	case "unknown":
		return 0.5
	default:
		return 0.0
	}
}

func getAIDataKeys(p *datastore.PollingEnt) []string {
	keys := []string{}
	if p.Type == "syslog" && p.Mode == "pri" {
		for i := 0; i < 256; i++ {
			keys = append(keys, fmt.Sprintf("pri_%d", i))
		}
		return keys
	}
	for k, v := range p.Result {
		if k == "lastTime" || strings.HasPrefix(k, "_") {
			continue
		}
		switch v.(type) {
		case float64, float32, int, int64, int32:
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// MakeAIData constructs the normalized feature DataFrame from polling logs.
func MakeAIData(p *datastore.PollingEnt, logs []*datastore.PollingLogEnt) (*AIDataFrame, error) {
	if p == nil {
		return nil, fmt.Errorf("no polling")
	}
	keys := getAIDataKeys(p)
	if len(keys) < 1 {
		return nil, fmt.Errorf("no numeric feature keys")
	}
	keys = append(keys, "state")

	df := &AIDataFrame{
		Time: []int64{},
		Data: make(map[string][]float64),
	}
	df.Data["hour"] = []float64{}
	df.Data["weekday"] = []float64{}
	for _, k := range keys {
		df.Data[k] = []float64{}
	}

	if len(logs) < 1 {
		return nil, fmt.Errorf("no logs")
	}

	// First pass: hourly window aggregation
	windowSec := int64(3600)
	st := windowSec * (time.Unix(0, logs[0].Time).Unix() / windowSec)
	ent := make(map[string]float64)
	maxVals := make(map[string]float64)
	for _, k := range keys {
		ent[k] = 0.0
		maxVals[k] = 0.0
	}
	var count float64

	for _, l := range logs {
		ct := windowSec * (time.Unix(0, l.Time).Unix() / windowSec)
		if st != ct {
			if count > 0.0 {
				ts := time.Unix(st, 0)
				df.Time = append(df.Time, ts.Unix())
				df.Data["hour"] = append(df.Data["hour"], float64(ts.Hour())/23.0)
				df.Data["weekday"] = append(df.Data["weekday"], float64(ts.Weekday())/6.0)
				for _, k := range keys {
					avg := ent[k] / count
					df.Data[k] = append(df.Data[k], avg)
					if maxVals[k] < avg {
						maxVals[k] = avg
					}
					ent[k] = 0.0
				}
			}
			st = ct
			count = 0.0
		}
		count += 1.0
		for _, k := range keys {
			if k == "state" {
				ent["state"] += getStateNum(l.State)
				continue
			}
			if v, ok := l.Result[k]; ok {
				switch fv := v.(type) {
				case float64:
					ent[k] += fv
				case float32:
					ent[k] += float64(fv)
				case int:
					ent[k] += float64(fv)
				case int64:
					ent[k] += float64(fv)
				}
			}
		}
	}
	if count > 0.0 {
		ts := time.Unix(st, 0)
		df.Time = append(df.Time, ts.Unix())
		df.Data["hour"] = append(df.Data["hour"], float64(ts.Hour())/23.0)
		df.Data["weekday"] = append(df.Data["weekday"], float64(ts.Weekday())/6.0)
		for _, k := range keys {
			avg := ent[k] / count
			df.Data[k] = append(df.Data[k], avg)
			if maxVals[k] < avg {
				maxVals[k] = avg
			}
		}
	}

	// If hourly aggregation produces fewer than 10 samples but we have >= 10 raw logs,
	// use raw log timestamps and values to allow prompt evaluation
	if df.Len() < 10 && len(logs) >= 10 {
		df.Time = []int64{}
		df.Data["hour"] = []float64{}
		df.Data["weekday"] = []float64{}
		for _, k := range keys {
			df.Data[k] = []float64{}
			maxVals[k] = 0.0
		}
		for _, l := range logs {
			ts := time.Unix(0, l.Time)
			df.Time = append(df.Time, ts.Unix())
			df.Data["hour"] = append(df.Data["hour"], float64(ts.Hour())/23.0)
			df.Data["weekday"] = append(df.Data["weekday"], float64(ts.Weekday())/6.0)
			for _, k := range keys {
				var val float64
				if k == "state" {
					val = getStateNum(l.State)
				} else if v, ok := l.Result[k]; ok {
					switch fv := v.(type) {
					case float64:
						val = fv
					case float32:
						val = float64(fv)
					case int:
						val = float64(fv)
					case int64:
						val = float64(fv)
					}
				}
				df.Data[k] = append(df.Data[k], val)
				if maxVals[k] < val {
					maxVals[k] = val
				}
			}
		}
	}

	// Normalize data
	for _, k := range keys {
		for j := range df.Data[k] {
			if maxVals[k] > 0.0 {
				df.Data[k][j] /= maxVals[k]
			} else {
				df.Data[k][j] = 0.0
			}
		}
	}

	// Filter by VectorCols if specified
	if p.VectorCols != "" {
		colMap := make(map[string]bool)
		for _, c := range strings.Split(p.VectorCols, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				colMap[c] = true
			}
		}
		for k := range df.Data {
			if !colMap[k] {
				delete(df.Data, k)
			}
		}
	}

	return df, nil
}

func getSampleData(req *AIReq) [][]float64 {
	cols := req.Df.ColumnNames()
	data := make([][]float64, req.Df.Len())
	for i := range data {
		data[i] = make([]float64, len(cols))
	}
	for i, col := range cols {
		if v, ok := req.Df.Data[col]; ok {
			for j, d := range v {
				data[j][i] = d
			}
		}
	}
	return data
}

func makeDeviationScore(req *AIReq, r []float64) *datastore.AIResultEnt {
	res := &datastore.AIResultEnt{
		PollingID: req.PollingID,
		ScoreData: [][]float64{},
	}
	if len(r) == 0 {
		return res
	}
	max, err := stats.Max(r)
	if err != nil {
		return res
	}
	min, err := stats.Min(r)
	if err != nil {
		return res
	}
	diff := max - min
	if diff == 0 {
		for i := range r {
			res.ScoreData = append(res.ScoreData, []float64{float64(req.Df.Time[i]), 50.0})
		}
		res.LastTime = req.Df.Time[len(req.Df.Time)-1]
		return res
	}
	normR := make([]float64, len(r))
	for i := range r {
		normR[i] = ((r[i] - min) / diff) * 100.0
	}
	mean, err := stats.Mean(normR)
	if err != nil {
		return res
	}
	sd, err := stats.StandardDeviation(normR)
	if err != nil || sd == 0 {
		for i := range r {
			res.ScoreData = append(res.ScoreData, []float64{float64(req.Df.Time[i]), 50.0})
		}
	} else {
		for i := range r {
			score := ((10.0 * (normR[i] - mean) / sd) + 50.0)
			res.ScoreData = append(res.ScoreData, []float64{float64(req.Df.Time[i]), score})
		}
	}
	res.LastTime = req.Df.Time[len(req.Df.Time)-1]
	return res
}

func calcIForest(req *AIReq) *datastore.AIResultEnt {
	sub := 256
	if req.Df.Len() < sub {
		sub = req.Df.Len() / 2
		if sub < 2 {
			sub = 2
		}
	}
	data := getSampleData(req)
	trees := 1000
	if req.Df.Len() < 50 {
		trees = 100
	}
	iforest, err := go_iforest.NewIForest(data, trees, sub)
	if err != nil {
		return &datastore.AIResultEnt{PollingID: req.PollingID}
	}
	r := make([]float64, len(data))
	for i, v := range data {
		r[i] = iforest.CalculateAnomalyScore(v)
	}
	return makeDeviationScore(req, r)
}

func calcHotelling(req *AIReq) *datastore.AIResultEnt {
	data := getSampleData(req)
	n := len(data)
	if n < 2 {
		return &datastore.AIResultEnt{PollingID: req.PollingID}
	}
	d := len(data[0])

	mean := make([]float64, d)
	for _, row := range data {
		for j, val := range row {
			mean[j] += val
		}
	}
	for j := range mean {
		mean[j] /= float64(n)
	}

	covData := make([]float64, d*d)
	for _, row := range data {
		for r := 0; r < d; r++ {
			for c := 0; c < d; c++ {
				covData[r*d+c] += (row[r] - mean[r]) * (row[c] - mean[c])
			}
		}
	}
	for i := range covData {
		covData[i] /= float64(n)
	}

	epsilon := 1e-6
	for r := 0; r < d; r++ {
		covData[r*d+r] += epsilon
	}

	cov := mat.NewDense(d, d, covData)
	var inv mat.Dense
	if err := inv.Inverse(cov); err != nil {
		return &datastore.AIResultEnt{PollingID: req.PollingID}
	}

	r := make([]float64, n)
	for i, row := range data {
		diff := make([]float64, d)
		for j := range row {
			diff[j] = row[j] - mean[j]
		}
		diffVec := mat.NewVecDense(d, diff)
		var tmp mat.VecDense
		tmp.MulVec(&inv, diffVec)
		r[i] = mat.Dot(diffVec, &tmp)
	}

	return makeDeviationScore(req, r)
}

func calcKNN(req *AIReq) *datastore.AIResultEnt {
	data := getSampleData(req)
	n := len(data)
	if n < 2 {
		return &datastore.AIResultEnt{PollingID: req.PollingID}
	}
	d := len(data[0])

	k := 5
	if k >= n {
		k = n - 1
	}
	if k < 1 {
		k = 1
	}

	r := make([]float64, n)
	for i := 0; i < n; i++ {
		dists := make([]float64, 0, n-1)
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			var sum float64
			for col := 0; col < d; col++ {
				diff := data[i][col] - data[j][col]
				sum += diff * diff
			}
			dists = append(dists, math.Sqrt(sum))
		}
		sort.Float64s(dists)

		var sumDist float64
		for idx := 0; idx < k; idx++ {
			sumDist += dists[idx]
		}
		r[i] = sumDist / float64(k)
	}

	return makeDeviationScore(req, r)
}

type statDetector struct {
	means []float64
	stds  []float64
	dim   int
}

func (sd *statDetector) fit(vectors [][]float64) error {
	n := len(vectors)
	if n == 0 {
		return errors.New("empty vectors")
	}
	dim := len(vectors[0])
	sd.dim = dim
	sd.means = make([]float64, dim)
	sd.stds = make([]float64, dim)

	for _, v := range vectors {
		for j := 0; j < dim; j++ {
			sd.means[j] += v[j]
		}
	}
	for j := 0; j < dim; j++ {
		sd.means[j] /= float64(n)
	}

	for _, v := range vectors {
		for j := 0; j < dim; j++ {
			diff := v[j] - sd.means[j]
			sd.stds[j] += diff * diff
		}
	}
	denom := float64(max(1, n-1))
	for j := 0; j < dim; j++ {
		variance := sd.stds[j] / denom
		if variance < 1e-8 {
			sd.stds[j] = 1.0
		} else {
			sd.stds[j] = math.Sqrt(variance)
		}
	}
	return nil
}

func (sd *statDetector) score(v []float64) float64 {
	if len(sd.means) == 0 {
		return 0.0
	}
	var sumSq float64
	for j := 0; j < sd.dim; j++ {
		val := 0.0
		if j < len(v) && !math.IsNaN(v[j]) && !math.IsInf(v[j], 0) {
			val = v[j]
		}
		z := (val - sd.means[j]) / sd.stds[j]
		sumSq += z * z
	}
	return math.Sqrt(sumSq / float64(sd.dim))
}

func calcStat(req *AIReq) *datastore.AIResultEnt {
	data := getSampleData(req)
	if len(data) < 2 {
		return &datastore.AIResultEnt{PollingID: req.PollingID}
	}
	detector := &statDetector{}
	if err := detector.fit(data); err != nil {
		return &datastore.AIResultEnt{PollingID: req.PollingID}
	}
	r := make([]float64, len(data))
	for i, v := range data {
		r[i] = detector.score(v)
	}
	return makeDeviationScore(req, r)
}

func calcMahalanobis(req *AIReq) *datastore.AIResultEnt {
	data := getSampleData(req)
	n := len(data)
	if n < 2 {
		return &datastore.AIResultEnt{PollingID: req.PollingID}
	}
	dim := len(data[0])

	mean := make([]float64, dim)
	for _, row := range data {
		for j, val := range row {
			mean[j] += val
		}
	}
	for j := range mean {
		mean[j] /= float64(n)
	}

	cov := make([]float64, dim*dim)
	for _, row := range data {
		for r := 0; r < dim; r++ {
			for c := 0; c < dim; c++ {
				cov[r*dim+c] += (row[r] - mean[r]) * (row[c] - mean[c])
			}
		}
	}
	denom := float64(max(1, n-1))
	for i := range cov {
		cov[i] /= denom
	}
	for r := 0; r < dim; r++ {
		cov[r*dim+r] += 1e-4
	}

	covMat := mat.NewDense(dim, dim, cov)
	var invMat mat.Dense
	if err := invMat.Inverse(covMat); err != nil {
		// Fallback to diagonal standard deviation
		r := make([]float64, n)
		for i, row := range data {
			var sumSq float64
			for j := 0; j < dim; j++ {
				diff := row[j] - mean[j]
				variance := cov[j*dim+j]
				if variance > 1e-8 {
					sumSq += (diff * diff) / variance
				}
			}
			r[i] = math.Sqrt(sumSq)
		}
		return makeDeviationScore(req, r)
	}

	r := make([]float64, n)
	for i, row := range data {
		diff := make([]float64, dim)
		for j := range row {
			diff[j] = row[j] - mean[j]
		}
		diffVec := mat.NewVecDense(dim, diff)
		var tmp mat.VecDense
		tmp.MulVec(&invMat, diffVec)
		distSq := mat.Dot(diffVec, &tmp)
		if distSq < 0 || math.IsNaN(distSq) {
			r[i] = 0.0
		} else {
			r[i] = math.Sqrt(distSq)
		}
	}
	return makeDeviationScore(req, r)
}

func calcLOF(req *AIReq) *datastore.AIResultEnt {
	data := getSampleData(req)
	if len(data) < 2 {
		return &datastore.AIResultEnt{PollingID: req.PollingID}
	}
	samples := lof.GetSamplesFromFloat64s(data)
	k := 5
	if k >= len(samples) {
		k = len(samples) - 1
	}
	if k < 1 {
		k = 1
	}
	lofGetter := lof.NewLOF(k)
	if err := lofGetter.Train(samples); err != nil {
		return &datastore.AIResultEnt{PollingID: req.PollingID}
	}
	r := make([]float64, len(samples))
	for i, s := range samples {
		r[i] = lofGetter.GetLOF(s, "fast")
	}
	return makeDeviationScore(req, r)
}

func calcAutoencoder(req *AIReq) *datastore.AIResultEnt {
	data := getSampleData(req)
	rows := len(data)
	if rows < 2 {
		return &datastore.AIResultEnt{PollingID: req.PollingID}
	}
	cols := len(data[0])

	// Normalize
	means := make([]float64, cols)
	stds := make([]float64, cols)
	for _, v := range data {
		for j, val := range v {
			means[j] += val
		}
	}
	for j := range means {
		means[j] /= float64(rows)
	}
	for _, v := range data {
		for j, val := range v {
			diff := val - means[j]
			stds[j] += diff * diff
		}
	}
	for j := range stds {
		variance := stds[j] / float64(rows)
		if variance < 1e-8 {
			stds[j] = 1.0
		} else {
			stds[j] = math.Sqrt(variance)
		}
	}

	normData := make([]tensai.Float, rows*cols)
	for i, v := range data {
		for j, val := range v {
			normVal := (val - means[j]) / stds[j]
			normData[i*cols+j] = tensai.Float(normVal)
		}
	}

	matData, err := tensai.NewMatrixFromSlice(rows, cols, normData)
	if err != nil {
		return calcStat(req)
	}

	bottleneck := cols / 4
	if bottleneck < 2 {
		bottleneck = 2
	}
	if bottleneck > 32 {
		bottleneck = 32
	}

	net := model.NewSequential()
	net.Add(layer.NewDense(bottleneck))
	net.Add(&layer.Tanh{})
	net.Add(layer.NewDense(cols))

	if err := net.Compile(cols, loss.MeanSquaredError{}, optim.NewAdam(0.01)); err != nil {
		return calcStat(req)
	}

	epochs := 20
	if rows > 500 {
		epochs = 10
	}
	for e := 1; e <= epochs; e++ {
		if _, err := net.FitStep(matData, matData); err != nil {
			return calcStat(req)
		}
	}

	r := make([]float64, rows)
	for i := 0; i < rows; i++ {
		rowSlice := normData[i*cols : (i+1)*cols]
		rowMat, err := tensai.NewMatrixFromSlice(1, cols, rowSlice)
		if err != nil {
			continue
		}
		pred, err := net.Predict(rowMat)
		if err != nil {
			continue
		}
		var mse float64
		for j := 0; j < cols; j++ {
			diff := float64(rowSlice[j] - pred.Data[j])
			mse += diff * diff
		}
		r[i] = mse / float64(cols)
	}

	return makeDeviationScore(req, r)
}

func calcLSTM(req *AIReq) *datastore.AIResultEnt {
	data := getSampleData(req)
	rows := len(data)
	if rows < 4 {
		return calcStat(req)
	}
	dim := len(data[0])

	samples := rows - 1
	inData := make([]tensai.Float, samples*dim)
	tgtData := make([]tensai.Float, samples*dim)
	for i := 0; i < samples; i++ {
		for j := 0; j < dim; j++ {
			inData[i*dim+j] = tensai.Float(data[i][j])
			tgtData[i*dim+j] = tensai.Float(data[i+1][j])
		}
	}

	inMat, err := tensai.NewMatrixFromSlice(samples, dim, inData)
	if err != nil {
		return calcStat(req)
	}
	tgtMat, err := tensai.NewMatrixFromSlice(samples, dim, tgtData)
	if err != nil {
		return calcStat(req)
	}

	hidden := dim / 4
	if hidden < 4 {
		hidden = 4
	}
	if hidden > 32 {
		hidden = 32
	}

	net := model.NewSequential()
	net.Add(layer.NewDense(hidden))
	net.Add(&layer.Tanh{})
	net.Add(layer.NewDense(dim))

	if err := net.Compile(dim, loss.MeanSquaredError{}, optim.NewAdam(0.01)); err != nil {
		return calcStat(req)
	}

	for e := 1; e <= 15; e++ {
		if _, err := net.FitStep(inMat, tgtMat); err != nil {
			return calcStat(req)
		}
	}

	r := make([]float64, rows)
	for i := 0; i < samples; i++ {
		rowSlice := inData[i*dim : (i+1)*dim]
		rowMat, err := tensai.NewMatrixFromSlice(1, dim, rowSlice)
		if err != nil {
			continue
		}
		pred, err := net.Predict(rowMat)
		if err != nil {
			continue
		}
		var mse float64
		for j := 0; j < dim; j++ {
			diff := float64(tgtData[i*dim+j] - pred.Data[j])
			mse += diff * diff
		}
		r[i+1] = mse / float64(dim)
	}
	r[0] = r[1]

	return makeDeviationScore(req, r)
}

// CalcAIScore calculates anomaly scores for an AIReq based on the selected algorithm.
func CalcAIScore(req *AIReq, aiMode string) *datastore.AIResultEnt {
	switch strings.ToLower(aiMode) {
	case "hotelling":
		return calcHotelling(req)
	case "mahalanobis":
		return calcMahalanobis(req)
	case "zscore", "stat":
		return calcStat(req)
	case "lof":
		return calcLOF(req)
	case "autoencoder", "ae":
		return calcAutoencoder(req)
	case "lstm":
		return calcLSTM(req)
	case "knn":
		return calcKNN(req)
	default:
		return calcIForest(req)
	}
}

// AnomalyService manages anomaly calculation background jobs and report queries.
type AnomalyService struct {
	store    datastore.DataStore
	logStore *parquet.Store
	lastCalc sync.Map
}

// NewAnomalyService creates a new AnomalyService instance.
func NewAnomalyService(store datastore.DataStore, logStore *parquet.Store) *AnomalyService {
	return &AnomalyService{
		store:    store,
		logStore: logStore,
	}
}

// Start begins periodic AI checks (once per minute) matching twsnmpfk backend.
func (s *AnomalyService) Start(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CheckAll(ctx)
		}
	}
}

// CheckAll checks all pollings with LogModeAI and updates their AIResult if needed.
func (s *AnomalyService) CheckAll(ctx context.Context) {
	if s.store == nil {
		return
	}
	s.store.ForEachPollings(func(pe *datastore.PollingEnt) bool {
		if pe.LogMode == datastore.LogModeAI {
			_ = s.ProcessPolling(ctx, pe, false)
		}
		return true
	})
}

// ProcessPolling processes AI anomaly detection for a single polling.
func (s *AnomalyService) ProcessPolling(ctx context.Context, pe *datastore.PollingEnt, force bool) error {
	if !force {
		// Check last time
		if v, ok := s.lastCalc.Load(pe.ID); ok {
			if lt, ok := v.(int64); ok && lt > time.Now().Unix()-3600 {
				return nil
			}
		}
		if last, err := s.store.GetAIResult(pe.ID); err == nil && last != nil {
			s.lastCalc.Store(pe.ID, last.LastTime)
			if last.LastTime > time.Now().Unix()-3600 {
				return nil
			}
		}
	}

	var logs []*datastore.PollingLogEnt
	if s.logStore != nil {
		l, err := s.logStore.GetAllPollingLog(ctx, pe.ID)
		if err == nil {
			logs = l
		}
	}

	df, err := MakeAIData(pe, logs)
	if err != nil || df == nil || df.Len() < 2 {
		return err
	}

	req := &AIReq{
		PollingID: pe.ID,
		Df:        *df,
	}
	res := CalcAIScore(req, pe.AIMode)
	if res == nil || len(res.ScoreData) < 1 {
		return fmt.Errorf("failed to calculate score")
	}

	s.lastCalc.Store(pe.ID, time.Now().Unix())
	if err := s.store.SaveAIResult(ctx, res); err != nil {
		return err
	}

	// Threshold evaluation & EventLog generation
	conf, _ := s.store.GetAIConf(ctx)
	if conf != nil && len(res.ScoreData) > 0 {
		lastScore := res.ScoreData[len(res.ScoreData)-1][1]
		level := ""
		if conf.HighThreshold > 0 && lastScore >= conf.HighThreshold {
			level = "high"
		} else if conf.LowThreshold > 0 && lastScore >= conf.LowThreshold {
			level = "low"
		} else if conf.WarnThreshold > 0 && lastScore >= conf.WarnThreshold {
			level = "warn"
		}
		if level != "" {
			nodeName := ""
			if n, err := s.store.GetNode(ctx, pe.NodeID); err == nil && n != nil {
				nodeName = n.Name
			}
			_ = s.store.AddEventLog(ctx, &datastore.EventLogEnt{
				Time:     time.Now().UnixNano(),
				Type:     "ai",
				Level:    level,
				NodeID:   pe.NodeID,
				NodeName: nodeName,
				Event:    fmt.Sprintf(i18n.Trans("AI report:%s(%s):%f"), pe.Name, pe.Type, lastScore),
			})
		}
	}

	return nil
}

// GetAIList returns the list of all pollings configured for AI anomaly detection with their scores.
func (s *AnomalyService) GetAIList(ctx context.Context) ([]*datastore.AIListEnt, error) {
	ret := make([]*datastore.AIListEnt, 0)
	if s.store == nil {
		return ret, nil
	}

	s.store.ForEachPollings(func(pe *datastore.PollingEnt) bool {
		if pe.LogMode != datastore.LogModeAI {
			return true
		}
		nodeName := ""
		if n, err := s.store.GetNode(ctx, pe.NodeID); err == nil && n != nil {
			nodeName = n.Name
		}

		air, err := s.store.GetAIResult(pe.ID)
		if err != nil || air == nil || len(air.ScoreData) < 1 {
			// Try on-demand calculation if not yet computed
			if pErr := s.ProcessPolling(ctx, pe, true); pErr == nil {
				air, _ = s.store.GetAIResult(pe.ID)
			}
		}

		if air != nil && len(air.ScoreData) > 0 {
			ret = append(ret, &datastore.AIListEnt{
				ID:       pe.ID,
				Node:     nodeName,
				Polling:  pe.Name,
				Score:    air.ScoreData[len(air.ScoreData)-1][1],
				Count:    len(air.ScoreData),
				LastTime: air.LastTime,
			})
		}
		return true
	})

	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Score > ret[j].Score
	})
	return ret, nil
}

// ExportAIData exports the feature data frame as CSV for a polling.
func (s *AnomalyService) ExportAIData(ctx context.Context, pollingID string, w io.Writer) error {
	pe, err := s.store.GetPolling(ctx, pollingID)
	if err != nil || pe == nil {
		return fmt.Errorf("polling not found")
	}
	var logs []*datastore.PollingLogEnt
	if s.logStore != nil {
		l, err := s.logStore.GetAllPollingLog(ctx, pe.ID)
		if err == nil {
			logs = l
		}
	}
	df, err := MakeAIData(pe, logs)
	if err != nil || df == nil {
		return err
	}
	return df.ToCSV(w)
}

// DeleteAIResult deletes AI result for an ID or all.
func (s *AnomalyService) DeleteAIResult(ctx context.Context, id string) error {
	if err := s.store.DeleteAIResult(ctx, id); err != nil {
		return err
	}
	if id == "all" {
		s.lastCalc = sync.Map{}
	} else {
		s.lastCalc.Delete(id)
	}
	_ = s.store.AddEventLog(ctx, &datastore.EventLogEnt{
		Time:  time.Now().UnixNano(),
		Type:  "user",
		Level: "info",
		Event: fmt.Sprintf(i18n.Trans("Delete AI Result(%s)"), id),
	})
	slog.Info("AI result deleted", "id", id)
	return nil
}
